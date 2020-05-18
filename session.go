package wirenet

import (
	"io"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

//
//import (
//	"context"
//	"io"
//	"log"
//	"net"
//	"sync"
//	"time"
//
//	"github.com/google/uuid"
//	"github.com/hashicorp/yamux"
//)
//

type Stream interface {
	io.Closer
	io.ReaderFrom
	io.WriterTo
	io.Writer
	io.Reader
}

type stream struct {
	name   string
	conn   *yamux.Stream
	closed bool
	buf    []byte
}

const bufSize = 1 << 8

func newStream(name string, conn *yamux.Stream) Stream {
	return &stream{
		name: name,
		conn: conn,
		buf:  make([]byte, bufSize),
	}
}

func (s *stream) Write(p []byte) (n int, err error) {
	if s.closed {
		return 0, ErrClosedCommand
	}
	return s.conn.Write(p)
}

func (s *stream) Read(p []byte) (n int, err error) {
	if s.closed {
		return 0, ErrClosedCommand
	}
	return s.conn.Read(p)
}

func (s *stream) WriteTo(w io.Writer) (n int64, err error) {
	s.buf = s.buf[:]
	for {
		if s.closed {
			return 0, ErrClosedCommand
		}
		nr, er := s.conn.Read(s.buf)
		if nr > 0 {
			nw, ew := w.Write(s.buf[0:nr])
			if nw > 0 {
				n += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}

		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return
}

func (s *stream) ReadFrom(r io.Reader) (n int64, err error) {
	s.buf = s.buf[:]
	for {
		if s.closed {
			return 0, ErrClosedCommand
		}
		nr, er := r.Read(s.buf)
		if nr > 0 {
			nw, ew := s.conn.Write(s.buf[0:nr])
			if nw > 0 {
				n += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}

		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return
}

func (s *stream) Close() error {
	if s.closed {
		return ErrClosedCommand
	}
	s.closed = true
	return s.conn.Close()
}

type Session interface {
	ID() uuid.UUID
	IsClosed() bool
	Close() error
	HasStream(name string) bool
	OpenStream(name string) (Stream, error)
}

type session struct {
	id       uuid.UUID
	conn     *yamux.Session
	w        *wire
	commands []string
}

func openSession(conn *yamux.Session, w *wire, commandNames []string) {
	sess := &session{
		id:       uuid.New(),
		conn:     conn,
		w:        w,
		commands: commandNames,
	}

	go sess.run()
}

func (s *session) HasStream(name string) bool {
	for _, cmd := range s.commands {
		if name == cmd {
			return true
		}
	}
	return false
}

func (s *session) dispatch(conn *yamux.Stream) {
	frm, err := recvFrame(conn, func(f frame) error {
		if s.w.verifyToken == nil {
			return nil
		}
		return s.w.verifyToken(f.Command(), f.Payload())
	})
	if err != nil {
		conn.Close()
		return
	}
	conn.Shrink()

	handler, ok := s.w.handlers[frm.Command()]
	if !ok {
		conn.Close()
		return
	}
	stream := newStream(frm.Command(), conn)
	handler(stream)

	stream.Close()
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

	return newStream(name, conn), nil
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
