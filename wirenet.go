package wirenet

import (
	"crypto/tls"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/hashicorp/yamux"
)

const (
	ClientSide Role = 1
	ServerSide Role = 2
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

type SessionHook func(uuid.UUID)
type RetryPolicy func(min, max time.Duration, attemptNum int) time.Duration

type Handler func(Stream)

type Streamer interface {
	Stream(name string) (Stream, error)
}

type Sessions map[uuid.UUID]Session

type Wire interface {
	Sessions() Sessions
	MountStream(name string, h Handler)
	OpenStream(name string) (Stream, error)
	Close() error
	Listen() error
}

const cmdSep = ";"

type wire struct {
	addr string

	readTimeout      time.Duration
	writeTimeout     time.Duration
	sessCloseTimeout time.Duration

	role          Role
	openSessHook  SessionHook
	closeSessHook SessionHook
	onListen      func(io.Closer)
	transportConf *yamux.Config

	closed  bool
	closeCh chan chan error
	waitCh  chan struct{}

	retryWaitMin time.Duration
	retryWaitMax time.Duration
	retryMax     int
	retryPolicy  RetryPolicy

	registerSess   chan Session
	unregisterSess chan Session
	sessions       Sessions

	token       []byte
	verifyToken func(string, []byte) error

	tlsConfig *tls.Config

	handlers map[string]Handler
	mu       sync.RWMutex
}

func New(addr string, role Role, opts ...Option) (Wire, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	if len(addr) == 0 {
		return nil, ErrAddrEmpty
	}
	wire := &wire{
		addr:     addr,
		handlers: make(map[string]Handler),

		readTimeout:      DefaultReadTimeout,
		writeTimeout:     DefaultWriteTimeout,
		sessCloseTimeout: DefaultSessionCloseTimeout,

		sessions:       make(Sessions),
		registerSess:   make(chan Session, 1),
		unregisterSess: make(chan Session, 1),

		role:          role,
		openSessHook:  func(_ uuid.UUID) {},
		closeSessHook: func(_ uuid.UUID) {},
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

	go wire.sessionManagement()

	return wire, nil
}

func NewServer(addr string, opts ...Option) (Wire, error) {
	return New(addr, ServerSide, opts...)
}

func NewClient(addr string, opts ...Option) (Wire, error) {
	return New(addr, ClientSide, opts...)
}

func (w *wire) OpenStream(name string) (Stream, error) {
	if len(w.sessions) == 0 || w.isClosed() {
		return nil, ErrSessionClosed
	}

	var sess Session
	isClientSide := w.role == ClientSide
	isServerSide := w.role == ServerSide
	for _, s := range w.sessions {
		if isClientSide || (isServerSide && s.HasStream(name)) {
			sess = s
			break
		}
	}
	if sess == nil {
		return nil, ErrSessionClosed
	}
	return sess.OpenStream(name)
}

func (w *wire) Sessions() Sessions {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sessions
}

func (w *wire) Listen() (err error) {
	switch w.role {
	case ClientSide:
		err = w.acceptClient()
	case ServerSide:
		err = w.acceptServer()
	}
	return err
}

func (w *wire) MountStream(name string, h Handler) {
	w.handlers[name] = h
}

//func (w *wire) VerifyToken(fn func(string, []byte) error) {
//	w.verifyToken = fn
//}

//func (w *wire) WithToken(token []byte) {
//	w.token = token
//}

func (w *wire) isClosed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.closed
}

func (w *wire) Close() (err error) {
	if w.isClosed() {
		return ErrWireClosed
	}

	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()

	errCh := make(chan error)
	w.closeCh <- errCh
	err = <-errCh

	return err
}

func (w *wire) acceptClient() (err error) {
	for i := 0; ; i++ {
		attemptNum := i
		if attemptNum > w.retryMax || w.isClosed() {
			break
		}

		var (
			conn net.Conn
			er   error
		)
		if w.tlsConfig != nil {
			conn, err = tls.Dial("tcp", w.addr, w.tlsConfig)
		} else {
			conn, er = net.Dial("tcp", w.addr)
		}
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

		if authErr := w.sendInitFrames(wrapConn); authErr != nil {
			err = authErr
			break
		}

		w.waitCh = make(chan struct{})

		go w.shutdown(wrapConn)
		if w.onListen != nil {
			w.onListen(w)
		}

		openSession(wrapConn, w, nil)

		<-w.waitCh
	}
	return err
}

func (w *wire) listen() (listener net.Listener, err error) {
	if w.tlsConfig != nil {
		listener, err = tls.Listen("tcp", w.addr, w.tlsConfig)
	} else {
		listener, err = net.Listen("tcp", w.addr)
	}
	return listener, err
}

func (w *wire) acceptServer() (err error) {
	listener, err := w.listen()
	if err != nil {
		return err
	}
	defer listener.Close()

	go w.shutdown(listener)
	if w.onListen != nil {
		w.onListen(w)
	}

	var closed bool
	for {
		closed = w.isClosed()
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			if !closed {
				err = acceptErr
			}
			break
		}

		// close all new connections
		if closed {
			_ = conn.Close()
			continue
		}

		wrapConn, serveErr := yamux.Server(conn, w.transportConf)
		if serveErr != nil {
			err = serveErr
			break
		}

		commandNames, authErr := w.recvInitFrames(wrapConn)
		if authErr != nil {
			_ = conn.Close()
			continue
		}

		openSession(wrapConn, w, commandNames)
	}
	return err
}

func (w *wire) shutdown(conn io.Closer) {
	errCh, ok := <-w.closeCh
	if !ok {
		return
	}

	//sessLen := w.sessHub.Len()
	//workerNum := sessLen
	//if sessLen > 1 {
	//	workerNum /= 2
	//}

	//var (
	//	queueCh = make(chan *session, workerNum)
	//	doneCh  = make(chan error, sessLen)
	//	closeCh = make(chan interface{})
	//)
	//
	//for i := 0; i < workerNum; i++ {
	//	go w.shutdownSession(queueCh, doneCh, closeCh)
	//}
	//for _, sess := range w.sessHub.List() {
	//	queueCh <- sess
	//}
	//
	shutdownErr := NewShutdownError()
	//for ei := 0; ei < sessLen; ei++ {
	//	err := <-doneCh
	//	if err != nil {
	//		shutdownErr.Errors = append(shutdownErr.Errors, err)
	//	}
	//}

	//close(closeCh)
	//
	if err := conn.Close(); err != nil {
		shutdownErr.Add(err)
	}
	if !shutdownErr.IsFilled() {
		shutdownErr = nil
	}

	if w.role == ClientSide {
		close(w.waitCh)
	}
	errCh <- shutdownErr
}

func (w *wire) sessionManagement() {
	for {
		select {
		case sess, ok := <-w.registerSess:
			if !ok {
				return
			}

			w.sessions[sess.ID()] = sess
			w.openSessHook(sess.ID())

		case sess, ok := <-w.unregisterSess:
			if !ok {
				return
			}

			w.closeSessHook(sess.ID())
			delete(w.sessions, sess.ID())
		}
	}
}

func (w *wire) sendInitFrames(conn *yamux.Session) error {
	stream, err := conn.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	auth, err := sendFrame(authorization, permFrameTyp, w.token, stream)
	if err != nil {
		return err
	}
	if auth.Command() != authorization && !auth.IsRecvFrame() {
		return io.ErrUnexpectedEOF
	}

	stream.Shrink()
	commandNames := make([]string, 0, len(w.handlers))
	for name, _ := range w.handlers {
		commandNames = append(commandNames, name)
	}
	payload := []byte(strings.Join(commandNames, cmdSep))
	cmd, err := sendFrame(commands, initFrameTyp, payload, stream)
	if err != nil {
		return err
	}
	if cmd.Command() != commands && !auth.IsRecvFrame() {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func (w *wire) recvInitFrames(conn *yamux.Session) ([]string, error) {
	stream, err := conn.AcceptStream()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	if _, err := recvFrame(stream, func(f frame) error {
		if w.verifyToken == nil {
			return nil
		}
		return w.verifyToken(f.Command(), f.Payload())
	}); err != nil {
		return nil, err
	}

	stream.Shrink()

	cmd, err := recvFrame(stream, nil)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(cmd.Payload()), cmdSep), nil
}

//func (w *wire) shutdownSession(q chan *session, e chan error, c chan interface{}) {
//	for {
//		select {
//		case sess, ok := <-q:
//			if !ok {
//				return
//			}
//			err := sess.Close()
//			if errors.Is(err, ErrSessionClosed) {
//				err = nil
//			}
//			e <- err
//		case <-c:
//			return
//		}
//	}
//}

func validateRole(r Role) error {
	switch r {
	case ClientSide, ServerSide:
		return nil
	default:
		return ErrUnknownListenerSide
	}
}
