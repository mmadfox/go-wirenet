package wirenet

import (
	"io"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

const bufSize = 1 << 9

type Stream interface {
	ID() uuid.UUID
	Session() Session
	Name() string
	IsClosed() bool

	io.Closer
	io.ReaderFrom
	io.WriterTo
	io.Writer
	io.Reader
}

type stream struct {
	id     uuid.UUID
	sess   *session
	name   string
	conn   *yamux.Stream
	closed bool
	buf    []byte
}

func openStream(sess *session, name string, conn *yamux.Stream) Stream {
	stream := &stream{
		id:   uuid.New(),
		sess: sess,
		name: name,
		conn: conn,
		buf:  make([]byte, bufSize),
	}
	sess.registerStream(stream)
	return stream
}

func (s *stream) IsClosed() bool {
	return s.closed
}

func (s *stream) ID() uuid.UUID {
	return s.id
}

func (s *stream) Session() Session {
	return s.sess
}

func (s *stream) Name() string {
	return s.name
}

func (s *stream) Write(p []byte) (n int, err error) {
	if s.closed {
		return 0, ErrStreamClosed
	}
	return s.conn.Write(p)
}

func (s *stream) Read(p []byte) (n int, err error) {
	if s.closed {
		return 0, ErrStreamClosed
	}
	return s.conn.Read(p)
}

func (s *stream) WriteTo(w io.Writer) (n int64, err error) {
	s.buf = s.buf[:]
	for {
		if s.closed {
			return 0, ErrStreamClosed
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
			return 0, ErrStreamClosed
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
	s.closed = true
	s.sess.unregisterStream(s)
	_ = s.conn.Close()
	return nil
}
