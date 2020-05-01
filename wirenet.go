package wirenet

import (
	"errors"
	"net"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/hashicorp/yamux"
)

const (
	ClientSide Role = 1
	ServerSide Role = 2
)

var (
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

	sessions map[uuid.UUID]*session
	isClosed bool
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
		sessions:      make(map[uuid.UUID]*session),
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

	go wire.sessionsManage()

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

// 1. Если сервер сторона, то запретить подключение
// 2. Закрыть все сессии и дождаться выполнение всех команд(стримов)
// 3. Закрыть слушателя и выйты

func (w *wire) Close() (err error) {
	w.isClosed = true

	switch w.role {
	case ClientSide:
		err = w.closeClientSide()
	case ServerSide:
		err = w.closeServerSide()
	}
	return err
}

func (w *wire) closeClientSide() error {
	return nil
}

func (w *wire) closeServerSide() error {
	return nil
}

func (w *wire) acceptServer() (err error) {
	listener, err := net.Listen("tcp", w.addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			err = acceptErr
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

		go newSession(w, conn, wrapConn).handle()
	}
	return err
}

//func (w *wire) openSession(conn net.Conn, transportSess *yamux.Session) {
//	log.Println("session G")
//	session := newSession(
//		conn,
//		transportSess,
//	)
//	defer func() {
//		w.unRegSessCh <- session
//		_ = transportSess.Close()
//		_ = conn.Close()
//		_ = w.closeSessHook(session)
//	}()
//	if err := w.openSessHook(session); err != nil {
//		return
//	}
//	w.regSessCh <- session
//}

func (w *wire) sessionsManage() {
	for {
		select {
		case sess := <-w.regSessCh:
			if err := w.openSessHook(sess); err != nil {
				_ = sess.Close()
				continue
			}
			w.sessions[sess.id] = sess
		case sess := <-w.unRegSessCh:
			_ = w.closeSessHook(sess)
			delete(w.sessions, sess.id)
		}
	}
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
