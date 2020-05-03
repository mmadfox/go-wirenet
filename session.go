package wirenet

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

type Session interface {
	ID() uuid.UUID
	IsClosed() bool
	Close() error
	Command(string) (Cmd, error)
}

const (
	waitShutdownInterval = 500 * time.Millisecond
)

type session struct {
	id           uuid.UUID
	conn         net.Conn
	ts           *yamux.Session
	wire         *wire
	cmdCounter   int32
	closeCh      chan chan error
	waitCh       chan interface{}
	isClosed     bool
	hub          *commands
	closeTimeout time.Duration
}

func newSession(w *wire, conn net.Conn, sess *yamux.Session) *session {
	s := &session{
		wire:         w,
		conn:         conn,
		ts:           sess,
		id:           uuid.New(),
		closeCh:      make(chan chan error),
		waitCh:       make(chan interface{}),
		hub:          newCommands(),
		closeTimeout: w.sessCloseTimeout,
	}
	w.hub.register(s)
	return s
}

func (s *session) ID() uuid.UUID {
	return s.id
}

func (s *session) IsClosed() bool {
	return s.isClosed
}

func (s *session) Close() error {
	if s.isClosed {
		return ErrSessionClosed
	}

	s.isClosed = true
	errCh := make(chan error)
	s.closeCh <- errCh

	s.wire.hub.unregister(s)

	return <-errCh
}

func (s *session) Command(name string) (Cmd, error) {
	stream, err := s.ts.OpenStream()
	if err != nil {
		return nil, err
	}

	_, err = sendInitFrame(name, []byte("JWT"), stream)
	if err != nil {
		return nil, err
	}

	return newCommand(name, s), nil
}

func (s *session) runCommand(ctx context.Context, stream *yamux.Stream) {
	frm, err := recvInitFrame(stream, func(f frame) error {
		// JWT verify
		return nil
	})
	if err != nil {
		_ = stream.Close()
		return
	}

	log.Println("runCommand", frm.Command(), string(frm.Payload()))

	cmd := newCommand("", s)
	defer func() {
		if err := recover(); err != nil {
			// TODO: send to error handler
		}
		_ = cmd.Close()
		stream.Close()
		log.Println("CLOSE")
	}()

	log.Println("startRunCommand", stream.StreamID())
	time.Sleep(10 * time.Second)
	log.Println("stopRunCommand", stream.StreamID())
	cmd.Close()
}

func (s *session) handle() {
	ctx := s.shutdown()

	defer func() {
		close(s.waitCh)
		_ = s.wire.closeSessHook(s)
	}()

	if err := s.wire.openSessHook(s); err != nil {
		return
	}

	for {
		conn, err := s.ts.AcceptStream()
		if err != nil {
			_ = s.Close()
			return
		}
		if s.isClosed || s.wire.isClosed {
			_ = conn.Close()
			continue
		}
		go s.runCommand(ctx, conn)
	}
}

func (s *session) shutdown() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case errCh, ok := <-s.closeCh:
				if !ok {
					return
				}
				timeout := time.Now().Add(s.closeTimeout)
				for {
					if s.hub.total() <= 0 {
						break
					}
					if timeout.Unix() <= time.Now().Unix() {
						s.forceCloseCommands()
						break
					}
					time.Sleep(waitShutdownInterval)
				}
				cancel()
				errCh <- s.ts.Close()
				close(errCh)
				return
			}
		}
	}()
	return ctx
}

func (s *session) forceCloseCommands() {
	for _, cmd := range s.hub.store {
		_ = cmd.Close()
	}
}

func RandomPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
