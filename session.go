package wirenet

import (
	"io"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

type Session interface {
	ID() uuid.UUID
	IsClosed() bool
	Close() error
	StreamNames() []string
	HasStream(name string) bool
	OpenStream(name string) (Stream, error)
}

type session struct {
	id          uuid.UUID
	conn        *yamux.Session
	w           *wire
	streamNames []string
}

func openSession(sid uuid.UUID, conn *yamux.Session, w *wire, streamNames []string) {
	sess := &session{
		id:          sid,
		conn:        conn,
		w:           w,
		streamNames: streamNames,
	}
	go sess.run()
}

func (s *session) StreamNames() []string {
	return s.streamNames
}

func (s *session) HasStream(name string) bool {
	for _, cmd := range s.streamNames {
		if name == cmd {
			return true
		}
	}
	return false
}

func (s *session) dispatch(conn *yamux.Stream) {
	defer func() {
		_ = conn.Close()
	}()
	frm, err := recvFrame(conn, func(f frame) error {
		if s.w.verifyToken == nil {
			return nil
		}
		return s.w.verifyToken(f.Command(), f.Payload())
	})
	if err != nil {
		return
	}
	conn.Shrink()

	handler, ok := s.w.handlers[frm.Command()]
	if !ok {
		return
	}
	stream := newStream(s.id, frm.Command(), conn)
	handler(stream)
	_ = stream.Close()
}

func (s *session) run() {
	defer func() {
		s.w.unregisterSess <- s
	}()

	s.w.registerSess <- s

	for {
		conn, err := s.conn.AcceptStream()
		if err != nil {
			// s.Close()
			return
		}
		// if s.isClosed || s.wire.isClosed {
		//_ = conn.Close()
		//continue
		//}
		// go s.runCommand(ctx, conn)
		go s.dispatch(conn)
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

func (s *session) OpenStream(name string) (Stream, error) {
	conn, err := s.conn.OpenStream()
	if err != nil {
		return nil, err
	}

	frm, err := sendFrame(name, permFrameTyp, s.w.token, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if frm.Command() != name && !frm.IsRecvFrame() {
		conn.Close()
		return nil, io.ErrUnexpectedEOF
	}

	conn.Shrink()

	return newStream(s.id, name, conn), nil
}

//
//const (
//	waitShutdownInterval = 500 * time.Millisecond
//)
//
//type session struct {
//	id           uuid.UUID
//	conn         net.Conn
//	ts           *yamux.Session
//	wire         *wire
//	cmdCounter   int32
//	closeCh      chan chan error
//	waitCh       chan interface{}
//	isClosed     bool
//	cmdHub       commandHub
//	closeTimeout time.Duration
//	openHook     SessionHook
//	closeHook    SessionHook
//	mu           sync.RWMutex
//	hub          sessionHub
//}
//
//func newSession(w *wire, conn net.Conn, sess *yamux.Session) *session {
//	s := &session{
//		wire:         w,
//		conn:         conn,
//		ts:           sess,
//		id:           uuid.New(),
//		closeCh:      make(chan chan error),
//		waitCh:       make(chan interface{}),
//		cmdHub:       newCommandHub(),
//		closeTimeout: w.sessCloseTimeout,
//		openHook:     w.openSessHook,
//		closeHook:    w.closeSessHook,
//		hub:          w.sessHub,
//	}
//
//	s.hub.Register(s)
//
//	return s
//}
//
//func (s *session) ID() uuid.UUID {
//	return s.id
//}
//
//func (s *session) IsClosed() bool {
//	s.mu.RLock()
//	defer s.mu.RUnlock()
//	return s.isClosed
//}
//
//func (s *session) Close() error {
//	if s.IsClosed() {
//		return ErrSessionClosed
//	}
//
//	s.mu.Lock()
//	s.isClosed = true
//	s.mu.Unlock()
//
//	errCh := make(chan error)
//	s.closeCh <- errCh
//
//	s.hub.Unregister(s)
//
//	return <-errCh
//}
//
//func (s *session) Command(name string) (Cmd, error) {
//	stream, err := s.ts.OpenStream()
//	if err != nil {
//		return nil, err
//	}
//
//	frm, err := sendFrame(
//		name,
//		initFrameTyp,
//		s.wire.token,
//		stream)
//	if err != nil {
//		stream.Close()
//		return nil, err
//	}
//
//	if frm.Command() != name && !frm.IsRecvFrame() {
//		stream.Close()
//		return nil, io.ErrUnexpectedEOF
//	}
//
//	stream.Shrink()
//
//	return newCommand(name, stream, s.cmdHub), nil
//}
//
//func (s *session) runCommand(ctx context.Context, stream *yamux.Stream) {
//	defer func() {
//		if err := recover(); err != nil {
//		}
//		_ = stream.Close()
//	}()
//
//	frm, err := recvFrame(stream, func(f frame) error {
//		if s.wire.verifyToken == nil {
//			return nil
//		}
//		return s.wire.verifyToken(f.Command(), f.Payload())
//	})
//	if err != nil {
//		return
//	}
//
//	handler, ok := s.wire.handlers[frm.Command()]
//	if !ok {
//		return
//	}
//
//	stream.Shrink()
//
//	cmd := newCommand(frm.Command(), stream, s.cmdHub)
//	if err := handler.Handle(cmd); err != nil {
//		// TODO: write error
//	}
//	_ = cmd.Close()
//}
//
//func (s *session) tokenVerification() (err error) {
//	switch s.wire.role {
//	case ClientSide:
//		err = s.tokenVerificationClientSide()
//		if err != nil {
//			s.wire.retryMax = -1
//		}
//	case ServerSide:
//		err = s.tokenVerificationServerSide()
//	}
//	return err
//}
//
//func (s *session) tokenVerificationClientSide() error {
//	if s.wire.token == nil {
//		return nil
//	}
//	stream, err := s.ts.OpenStream()
//	if err != nil {
//		return err
//	}
//	defer stream.Close()
//
//	_, sendErr := sendFrame(
//		tokenVerification,
//		permFrameTyp,
//		s.wire.token,
//		stream)
//	if sendErr != nil {
//		err = sendErr
//	}
//	return err
//}
//
//func (s *session) tokenVerificationServerSide() error {
//	if s.wire.verifyToken == nil {
//		return nil
//	}
//
//	conn, err := s.ts.AcceptStream()
//	if err != nil {
//		return err
//	}
//	defer conn.Close()
//
//	_, err = recvFrame(conn, func(f frame) error {
//		return s.wire.verifyToken(f.Command(), f.Payload())
//	})
//	return err
//}
//
//func (s *session) serve() {
//	defer func() {
//		close(s.waitCh)
//		_ = s.closeHook(s)
//	}()
//
//	ctx := s.shutdown()
//
//	if err := s.tokenVerification(); err != nil {
//		_ = s.Close()
//		return
//	}
//
//	if err := s.openHook(s); err != nil {
//		_ = s.Close()
//		return
//	}
//
//	for {
//		log.Println("accept")
//		conn, err := s.ts.AcceptStream()
//		if err != nil {
//			_ = s.Close()
//			return
//		}
//		if s.isClosed || s.wire.isClosed {
//			_ = conn.Close()
//			continue
//		}
//		go s.runCommand(ctx, conn)
//	}
//}
//
//func (s *session) shutdown() context.Context {
//	ctx, cancel := context.WithCancel(context.Background())
//
//	go func() {
//		for {
//			select {
//			case <-s.waitCh:
//				return
//			case errCh, ok := <-s.closeCh:
//				if !ok {
//					return
//				}
//				timeout := time.Now().Add(s.closeTimeout)
//				for {
//					if s.cmdHub.Len() <= 0 {
//						break
//					}
//					if timeout.Unix() <= time.Now().Unix() {
//						s.cmdHub.Close()
//						break
//					}
//					time.Sleep(waitShutdownInterval)
//				}
//				cancel()
//				errCh <- s.ts.Close()
//				close(errCh)
//				return
//			}
//		}
//	}()
//	return ctx
//}
