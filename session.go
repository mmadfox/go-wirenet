package wirenet

import (
	"context"
	"io"
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
	cmdHub       commandHub
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
		cmdHub:       newCommandHub(),
		closeTimeout: w.sessCloseTimeout,
	}

	w.sessHub.Register(s)

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

	s.wire.sessHub.Unregister(s)

	return <-errCh
}

func (s *session) Command(name string) (Cmd, error) {
	stream, err := s.ts.OpenStream()
	if err != nil {
		return nil, err
	}

	frm, err := sendFrame(
		name,
		initFrameTyp,
		s.wire.token,
		stream)
	if err != nil {
		stream.Close()
		return nil, err
	}

	if frm.Command() != name && !frm.IsRecvFrame() {
		stream.Close()
		return nil, io.ErrUnexpectedEOF
	}

	stream.Shrink()

	return newCommand(name, stream, s.cmdHub), nil
}

func (s *session) runCommand(ctx context.Context, stream *yamux.Stream) {
	defer func() {
		if err := recover(); err != nil {
		}
		_ = stream.Close()
	}()

	frm, err := recvFrame(stream, func(f frame) error {
		if s.wire.verifyToken == nil {
			return nil
		}
		return s.wire.verifyToken(f.Command(), f.Payload())
	})
	if err != nil {
		return
	}

	handler, ok := s.wire.handlers[frm.Command()]
	if !ok {
		return
	}

	cmd := newCommand(frm.Command(), stream, s.cmdHub)
	if err := handler.Serve(cmd); err != nil {
		// TODO: write error
	}
	_ = cmd.Close()
}

func (s *session) tokenVerification() (err error) {
	switch s.wire.role {
	case ClientSide:
		err = s.tokenVerificationClientSide()
		if err != nil {
			s.wire.retryMax = -1
		}
	case ServerSide:
		err = s.tokenVerificationServerSide()
	}
	return err
}

func (s *session) tokenVerificationClientSide() error {
	if s.wire.token == nil {
		return nil
	}
	stream, err := s.ts.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	_, sendErr := sendFrame(
		tokenVerification,
		permFrameTyp,
		s.wire.token,
		stream)
	if sendErr != nil {
		err = sendErr
	}
	return err
}

func (s *session) tokenVerificationServerSide() error {
	if s.wire.verifyToken == nil {
		return nil
	}

	conn, err := s.ts.AcceptStream()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = recvFrame(conn, func(f frame) error {
		return s.wire.verifyToken(f.Command(), f.Payload())
	})
	return err
}

func (s *session) handle() {
	defer func() {
		close(s.waitCh)
		_ = s.wire.closeSessHook(s)
	}()

	ctx := s.shutdown()

	if err := s.tokenVerification(); err != nil {
		_ = s.Close()
		return
	}

	if err := s.wire.openSessHook(s); err != nil {
		_ = s.Close()
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
			case <-s.waitCh:
				return
			case errCh, ok := <-s.closeCh:
				if !ok {
					return
				}
				timeout := time.Now().Add(s.closeTimeout)
				for {
					if s.cmdHub.Len() <= 0 {
						break
					}
					if timeout.Unix() <= time.Now().Unix() {
						s.cmdHub.Close()
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
