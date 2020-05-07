package wirenet

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"time"

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

type SessionHook func(Session) error
type RetryPolicy func(min, max time.Duration, attemptNum int) time.Duration

type Handler interface {
	Name() string
	Serve(Cmd) error
}

type Wire interface {
	OpenSession(SessionHook)
	CloseSession(SessionHook)

	Mount(Handler)

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

	sessHub  sessionHub
	isClosed bool
	closeCh  chan chan error

	retryWaitMin time.Duration
	retryWaitMax time.Duration
	retryMax     int
	retryPolicy  RetryPolicy

	token       []byte
	verifyToken func(string, []byte) error

	tlsConfig *tls.Config

	handlers map[string]Handler
}

func New(addr string, role Role, opts ...Option) (Wire, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	if len(addr) == 0 {
		return nil, ErrListenerAddrEmpty
	}
	wire := &wire{
		addr:     addr,
		handlers: make(map[string]Handler),

		readTimeout:      DefaultReadTimeout,
		writeTimeout:     DefaultWriteTimeout,
		sessCloseTimeout: DefaultSessionCloseTimeout,

		role:          role,
		openSessHook:  defaultSessionHook,
		closeSessHook: defaultSessionHook,
		sessHub:       newSessionHub(),
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

func (w *wire) Mount(h Handler) {
	w.handlers[h.Name()] = h
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

		session := newSession(w, conn, wrapConn)
		go session.handle()
		go w.shutdown(wrapConn)

		<-session.waitCh
	}
	return err
}

func (w *wire) acceptServer() (err error) {
	var listener net.Listener
	if w.tlsConfig != nil {
		listener, err = tls.Listen("tcp", w.addr, w.tlsConfig)
	} else {
		listener, err = net.Listen("tcp", w.addr)
	}
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

	sessLen := w.sessHub.Len()
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
	w.sessHub.Each(func(sess *session) {
		queueCh <- sess
	})

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
