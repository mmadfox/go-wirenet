package wirenet

import (
	"errors"
	"io"
	"log"
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

type Wire interface {
	OpenSession(SessionHook)
	CloseSession(SessionHook)
	Listen() error
	Mount(string, func(Cmd)) error
	Close() error
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

	regSessCh   chan *session
	unRegSessCh chan *session

	hub      *sessions
	isClosed bool
	closeCh  chan chan error
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
		regSessCh:     make(chan *session, 1),
		unRegSessCh:   make(chan *session, 1),
		openSessHook:  defaultSessionHook,
		closeSessHook: defaultSessionHook,
		hub:           newSessions(),
		closeCh:       make(chan chan error),

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

	// go wire.sessionsManage()

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

func (w *wire) Close() (err error) {
	if w.isClosed {
		return ErrWireClosed
	}
	w.isClosed = true

	errCh := make(chan error)
	w.closeCh <- errCh
	return <-errCh
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
			err = acceptErr
			break
		}
		if w.isClosed {
			log.Println("reject CONN")
			_ = conn.Close()
			continue
		}

		wrapConn, serveErr := yamux.Server(conn, w.transportConf)
		if serveErr != nil {
			err = serveErr
			break
		}

		go newSession(w, conn, wrapConn).handle()
	}
	return err
}

func (w *wire) shutdown(conn io.Closer) {
	errCh, ok := <-w.closeCh
	if !ok {
		return
	}

	var (
		workerNum = 8
		sessLen   = w.hub.len()
		queueCh   = make(chan *session, workerNum)
		doneCh    = make(chan error, sessLen)
		closeCh   = make(chan interface{})
	)
	for w := 0; w < workerNum; w++ {
		go func(q chan *session, d chan error, wid int) {
			for {
				select {
				case sess, ok := <-queueCh:
					if !ok {
						return
					}
					err := sess.Close()
					if errors.Is(err, ErrSessionClosed) {
						err = nil
					}
					doneCh <- err
				case <-closeCh:
					return
				}
			}
		}(queueCh, doneCh, w)
	}
	for _, sess := range w.hub.store {
		queueCh <- sess
	}

	shutdownErr := &ShutdownError{
		Errors: make([]error, 0, 8),
	}
	for ei := 0; ei < sessLen; ei++ {
		err := <-doneCh
		if err != nil {
			shutdownErr.Errors = append(shutdownErr.Errors, err)
		}
	}
	close(closeCh)
	if len(shutdownErr.Errors) == 0 {
		shutdownErr = nil
	}
	errCh <- shutdownErr
}

func (w *wire) acceptClient() error {
	return nil
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

func (e *ShutdownError) Error() string {
	return "shutdown error"
}
