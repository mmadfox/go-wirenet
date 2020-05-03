package wirenet

import (
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/hashicorp/yamux"
)

const (
	ClientSide Role = 1
	ServerSide Role = 2
)

var (
	ErrWireClosed          = errors.New("wirenet closed")
	ErrListenerAddrEmpty   = errors.New("wirenet: listener address is empty")
	ErrUnknownListenerSide = errors.New("wirenet: unknown role listener")
	ErrSessionClosed       = errors.New("wirenet: session closed")
)

type Role int

func (s Role) String() (side string) {
	switch s {
	case ClientSide:
		side = "client side wire"
	case ServerSide:
		side = "server side wire"
	default:
		side = "unknown"
	}
	return side
}

type SessionHook func(Session) error
type RetryPolicy func(min, max time.Duration, attemptNum int) time.Duration

type Wire interface {
	OpenSession(SessionHook)
	CloseSession(SessionHook)

	Mount(string, func(Cmd)) error

	VerifyToken(func(string, []byte) error)
	WithToken([]byte)

	Close() error
	Listen() error
}

var defaultSessionHook = func(s Session) error { return nil }

type wire struct {
	addr string

	readTimeout      time.Duration
	writeTimeout     time.Duration
	sessCloseTimeout time.Duration

	role          Role
	openSessHook  SessionHook
	closeSessHook SessionHook
	transportConf *yamux.Config

	hub      *sessions
	isClosed bool
	closeCh  chan chan error

	retryWaitMin time.Duration
	retryWaitMax time.Duration
	retryMax     int
	retryPolicy  RetryPolicy

	token       []byte
	verifyToken func(string, []byte) error
}

func New(addr string, role Role, opts ...Option) (Wire, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	if len(addr) == 0 {
		return nil, ErrListenerAddrEmpty
	}
	wire := &wire{
		addr: addr,

		readTimeout:      DefaultReadTimeout,
		writeTimeout:     DefaultWriteTimeout,
		sessCloseTimeout: DefaultSessionCloseTimeout,

		role:          role,
		openSessHook:  defaultSessionHook,
		closeSessHook: defaultSessionHook,
		hub:           newSessions(),
		closeCh:       make(chan chan error),

		retryMax:     DefaultRetryMax,
		retryWaitMin: DefaultRetryWaitMin,
		retryWaitMax: DefaultRetryWaitMax,
		retryPolicy:  DefaultRetryPolicy,

		transportConf: &yamux.Config{
			AcceptBacklog:          DefaultAcceptBacklog,
			EnableKeepAlive:        DefaultEnableKeepAlive,
			KeepAliveInterval:      DefaultKeepAliveInterval,
			ConnectionWriteTimeout: DefaultWriteTimeout,
			MaxStreamWindowSize:    DefaultAcceptBacklog * 1024,
			LogOutput:              os.Stderr,
		},
	}
	for _, opt := range opts {
		opt(wire)
	}

	return wire, nil
}

func (w *wire) OpenSession(hook SessionHook) {
	w.openSessHook = hook
}

func (w *wire) CloseSession(hook SessionHook) {
	w.closeSessHook = hook
}

func (w *wire) Listen() (err error) {
	switch w.role {
	case ClientSide:
		err = w.acceptClient()
	case ServerSide:
		err = w.acceptServer()
	default:
		err = ErrUnknownListenerSide
	}
	return err
}

func (w *wire) Mount(name string, handler func(Cmd)) error {
	return nil
}

func (w *wire) VerifyToken(fn func(string, []byte) error) {
	w.verifyToken = fn
}

func (w *wire) WithToken(token []byte) {
	w.token = token
}

func (w *wire) Close() (err error) {
	if w.isClosed {
		return ErrWireClosed
	}
	w.isClosed = true

	errCh := make(chan error)
	w.closeCh <- errCh
	return <-errCh
}

func (w *wire) acceptClient() (err error) {
	for i := 0; ; i++ {
		attemptNum := i
		if attemptNum > w.retryMax {
			break
		}

		conn, er := net.Dial("tcp", w.addr)
		if er != nil {
			er = err
			retryWait := w.retryPolicy(
				w.retryWaitMin,
				w.retryWaitMax,
				attemptNum)
			time.Sleep(retryWait)
			continue
		}

		wrapConn, serveErr := yamux.Client(conn, w.transportConf)
		if serveErr != nil {
			err = serveErr
			break
		}

		session := newSession(w, conn, wrapConn)
		go session.handle()
		go w.shutdown(wrapConn)

		<-session.waitCh
	}
	return err
}

func (w *wire) acceptServer() (err error) {
	listener, err := net.Listen("tcp", w.addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	go w.shutdown(listener)

	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			if !w.isClosed {
				err = acceptErr
			}
			break
		}
		if w.isClosed {
			_ = conn.Close()
			continue
		}

		wrapConn, serveErr := yamux.Server(conn, w.transportConf)
		if serveErr != nil {
			err = serveErr
			break
		}

		session := newSession(w, conn, wrapConn)
		go session.handle()
	}
	return err
}

func (w *wire) shutdown(conn io.Closer) {
	errCh, ok := <-w.closeCh
	if !ok {
		return
	}

	sessLen := w.hub.len()
	workerNum := sessLen
	if sessLen > 1 {
		workerNum /= 2
	}

	var (
		queueCh = make(chan *session, workerNum)
		doneCh  = make(chan error, sessLen)
		closeCh = make(chan interface{})
	)

	for i := 0; i < workerNum; i++ {
		go w.shutdownSession(queueCh, doneCh, closeCh)
	}
	for _, sess := range w.hub.store {
		queueCh <- sess
	}

	shutdownErr := NewShutdownError()
	for ei := 0; ei < sessLen; ei++ {
		err := <-doneCh
		if err != nil {
			shutdownErr.Errors = append(shutdownErr.Errors, err)
		}
	}

	close(closeCh)

	if err := conn.Close(); err != nil {
		shutdownErr.Errors = append(shutdownErr.Errors, err)
	}
	if !shutdownErr.HasErrors() {
		shutdownErr = nil
	}
	errCh <- shutdownErr
}

func (w *wire) shutdownSession(q chan *session, e chan error, c chan interface{}) {
	for {
		select {
		case sess, ok := <-q:
			if !ok {
				return
			}
			err := sess.Close()
			if errors.Is(err, ErrSessionClosed) {
				err = nil
			}
			e <- err
		case <-c:
			return
		}
	}
}

func validateRole(r Role) error {
	switch r {
	case ClientSide, ServerSide:
		return nil
	default:
		return ErrUnknownListenerSide
	}
}

type sessions struct {
	store map[uuid.UUID]*session
	sync.RWMutex
	counter int
}

func newSessions() *sessions {
	return &sessions{
		store: make(map[uuid.UUID]*session),
	}
}

func (s *sessions) len() int {
	s.RLock()
	defer s.RUnlock()
	return s.counter
}

func (s *sessions) register(sess *session) {
	s.Lock()
	defer s.Unlock()
	s.store[sess.id] = sess
	s.counter++
}

func (s *sessions) unregister(sess *session) {
	s.Lock()
	defer s.Unlock()
	delete(s.store, sess.id)
	if s.counter > 0 {
		s.counter--
	}
}

type ShutdownError struct {
	Errors []error
}

func NewShutdownError() *ShutdownError {
	return &ShutdownError{
		Errors: make([]error, 0, 8),
	}
}

func (e *ShutdownError) HasErrors() bool {
	return len(e.Errors) > 0
}

func (e *ShutdownError) Error() string {
	return "shutdown error"
}
