package wirenet

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

type Session interface {
	ID() uuid.UUID
	IsClosed() bool
	Close() error
	StreamNames() []string
	OpenStream(name string) (Stream, error)
}

type session struct {
	id               uuid.UUID
	conn             *yamux.Session
	w                *wire
	streamNames      []string
	closed           bool
	closeCh          chan chan error
	activeStreams    uint32
	streams          map[uuid.UUID]Stream
	registerStream   chan Stream
	unregisterStream chan Stream
	mu               sync.RWMutex
}

func openSession(sid uuid.UUID, conn *yamux.Session, w *wire, streamNames []string) {
	sess := &session{
		id:               sid,
		conn:             conn,
		w:                w,
		streamNames:      streamNames,
		closeCh:          make(chan chan error),
		streams:          make(map[uuid.UUID]Stream),
		registerStream:   make(chan Stream),
		unregisterStream: make(chan Stream),
	}
	go sess.open()
}

func (s *session) StreamNames() []string {
	return s.streamNames
}

func (s *session) shutdown() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case stream, ok := <-s.registerStream:
				if !ok {
					return
				}

				s.streams[stream.ID()] = stream

			case stream, ok := <-s.unregisterStream:
				if !ok {
					return
				}

				delete(s.streams, stream.ID())

			case errCh, ok := <-s.closeCh:
				if !ok {
					return
				}
				timeout := time.Now().Add(time.Minute)
				for {
					activeStreams := atomic.LoadUint32(&s.activeStreams)
					if activeStreams <= 0 {
						break
					}
					if timeout.Unix() <= time.Now().Unix() {
						s.mu.RLock()
						for _, stream := range s.streams {
							stream.Close()
						}
						s.mu.RUnlock()
					}
					time.Sleep(300 * time.Millisecond)
				}
				cancel()
				errCh <- s.conn.Close()
				close(errCh)
				return
			}
		}
	}()
	return ctx
}

func (s *session) dispatchStream(ctx context.Context, conn *yamux.Stream) {
	defer func() {
		atomic.CompareAndSwapUint32(&s.activeStreams, s.activeStreams, s.activeStreams-1)
		_ = conn.Close()
	}()

	atomic.AddUint32(&s.activeStreams, 1)

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
	s.registerStream <- stream

	handler(stream)
	_ = stream.Close()

	s.unregisterStream <- stream
}

func (s *session) open() {
	defer func() {
		s.w.unregisterSess <- s
	}()

	ctx := s.shutdown()

	s.w.registerSess <- s

	for {
		conn, err := s.conn.AcceptStream()
		if err != nil {
			s.Close()
			return
		}

		s.mu.RLock()
		closed := s.closed
		s.mu.RUnlock()
		if closed || s.w.isClosed() {
			_ = conn.Close()
			continue
		}

		go s.dispatchStream(ctx, conn)
	}
}

func (s *session) ID() uuid.UUID {
	return s.id
}

func (s *session) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *session) Close() error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSessionClosed
	}
	s.mu.RUnlock()

	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	errCh := make(chan error)
	s.closeCh <- errCh

	return <-errCh
}

func (s *session) OpenStream(name string) (Stream, error) {
	if s.IsClosed() {
		return nil, ErrSessionClosed
	}

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
