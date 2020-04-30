package wirenet

import (
	"errors"
	"net"
	"time"

	"github.com/hashicorp/yamux"
)

const (
	ClientSide Role = 1
	ServerSide Role = 2
)

var (
	ErrUnknownListenerSide = errors.New("wirenet: unknown role listener")
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

type Listener interface {
	OpenSession(SessionHook)
	CloseSession(SessionHook)
	Listen() error
	Mount(string, func(Cmd)) error
	Close() error
}

type Wire struct {
	addr          string
	readTimeout   time.Duration
	writeTimeout  time.Duration
	role          Role
	openSessHook  SessionHook
	closeSessHook SessionHook
	transportConf *yamux.Config
}

func New(addr string, role Role, opts ...Option) (Listener, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	wire := &Wire{
		addr:         addr,
		readTimeout:  DefaultReadTimeout,
		writeTimeout: DefaultWriteTimeout,
		role:         role,
		transportConf: &yamux.Config{
			AcceptBacklog:          DefaultAcceptBacklog,
			EnableKeepAlive:        DefaultEnableKeepAlive,
			KeepAliveInterval:      DefaultKeepAliveInterval,
			ConnectionWriteTimeout: DefaultWriteTimeout,
			MaxStreamWindowSize:    DefaultAcceptBacklog * 1024,
		},
	}
	for _, opt := range opts {
		opt(wire)
	}
	return wire, nil
}

func (w *Wire) OpenSession(hook SessionHook) {
	w.openSessHook = hook
}

func (w *Wire) CloseSession(hook SessionHook) {
	w.closeSessHook = hook
}

func (w *Wire) Listen() (err error) {
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

func (w *Wire) Mount(name string, handler func(Cmd)) error {
	return nil
}

func (w *Wire) Close() error {
	return nil
}

func (w *Wire) acceptServer() error {
	listen, err := net.Listen("tcp", w.addr)
	if err != nil {
		return err
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			// TODO: handle error
			continue
		}
		transportSess, err := yamux.Server(conn, w.transportConf)
		if err != nil {
			return err
		}
		go w.openSession(conn, transportSess)
	}
	return nil
}

func (w *Wire) openSession(conn net.Conn, sess *yamux.Session) {

}

func (w *Wire) acceptClient() error {
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
