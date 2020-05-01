package wirenet

import (
	"context"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

type Session interface {
	ID() uuid.UUID
	IsClosed() bool
	Close() error
	Command(string) Cmd
}

type session struct {
	id           uuid.UUID
	conn         net.Conn
	ts           *yamux.Session
	wire         *wire
	cmdCounter   int32
	closeCh      chan chan error
	isClosed     bool
	hub          *commands
	closeTimeout time.Duration
}

func newSession(w *wire, conn net.Conn, sess *yamux.Session) *session {
	return &session{
		wire:         w,
		conn:         conn,
		ts:           sess,
		id:           uuid.New(),
		closeCh:      make(chan chan error),
		hub:          newCommands(),
		closeTimeout: w.sessCloseTimeout,
	}
}

func (s *session) ID() uuid.UUID {
	return s.id
}

func (s *session) IsClosed() bool {
	return false
}

func (s *session) Close() error {
	if s.isClosed {
		return ErrSessionClosed
	}

	s.isClosed = true
	errCh := make(chan error)
	s.closeCh <- errCh

	return <-errCh
}

func (s *session) Command(name string) Cmd {
	return newCommand(name, s)
}

func (s *session) runCommand(ctx context.Context, conn *yamux.Stream) {

}

func (s *session) handle() {
	ctx := s.shutdown()

	for {
		conn, err := s.ts.AcceptStream()
		if err != nil {
			_ = s.Close()
			return
		}
		if s.isClosed {
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
			case <-ctx.Done():
				return
			case errCh, ok := <-s.closeCh:
				if !ok {
					return
				}
				now := time.Now()
				for {
					if s.hub.total() <= 0 {
						break
					}
					if time.Since(now).Seconds() > s.closeTimeout.Seconds() {
						break
					}
					time.Sleep(time.Second)
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
