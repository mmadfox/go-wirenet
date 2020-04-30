package wirenet

import (
	"log"
	"net"

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
	id   uuid.UUID
	conn net.Conn
	ts   *yamux.Session
	wire *wire
}

func newSession(w *wire, conn net.Conn, sess *yamux.Session) *session {
	return &session{
		wire: w,
		conn: conn,
		ts:   sess,
		id:   uuid.New(),
	}
}

func (s *session) ID() uuid.UUID {
	return s.id
}

func (s *session) IsClosed() bool {
	return false
}

func (s *session) Close() error {
	return nil
}

func (s *session) Command(string) Cmd {
	return nil
}

func (s *session) handle() {
	s.wire.regSessCh <- s
	defer func() {
		s.wire.unRegSessCh <- s
	}()
	log.Println("handle", s.id)
}
