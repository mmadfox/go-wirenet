package wirenet

import (
	"io"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

const bufSize = 1 << 8

type Stream interface {
	SessionID() uuid.UUID
	Name() string

	io.Closer
	io.ReaderFrom
	io.WriterTo
	io.Writer
	io.Reader
}

type stream struct {
	sid    uuid.UUID
	name   string
	conn   *yamux.Stream
	closed bool
	buf    []byte
}

func newStream(sessID uuid.UUID, name string, conn *yamux.Stream) Stream {
	return &stream{
		sid:  sessID,
		name: name,
		conn: conn,
		buf:  make([]byte, bufSize),
	}
}

func (s *stream) SessionID() uuid.UUID {
	return s.sid
}

func (s *stream) Name() string {
	return s.name
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
