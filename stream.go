package wirenet

import (
	"encoding/binary"
	"io"
	"sync"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

const bufSize = 1 << 10
const eof = uint32(0)

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
	hdr    []byte
	mu     sync.RWMutex
}

func openStream(sess *session, name string, conn *yamux.Stream) Stream {
	stream := &stream{
		id:   uuid.New(),
		sess: sess,
		name: name,
		conn: conn,
		buf:  make([]byte, bufSize),
		hdr:  make([]byte, hdrLen),
	}

	sess.registerStream(stream)

	return stream
}

func (s *stream) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *stream) ID() uuid.UUID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

func (s *stream) Session() Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sess
}

func (s *stream) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.name
}

func (s *stream) Write(p []byte) (n int, err error) {
	if s.closed && s.sess.IsClosed() {
		return 0, ErrStreamClosed
	}
	return s.conn.Write(p)
}

func (s *stream) Read(p []byte) (n int, err error) {
	if s.closed && s.sess.IsClosed() {
		return 0, ErrStreamClosed
	}
	return s.conn.Read(p)
}

func (s *stream) writeEOF() error {
	if err := binary.Write(s.conn, binary.LittleEndian, eof); err != nil {
		return err
	}
	return nil
}

func (s *stream) writeHdr(size int) error {
	if err := binary.Write(s.conn, binary.LittleEndian, uint32(size)); err != nil {
		return err
	}
	return nil
}

func (s *stream) readHeader() (size int, err error) {
	n, err := s.conn.Read(s.hdr)
	if err != nil {
		return size, err
	}
	if n != hdrLen {
		return size, io.ErrShortBuffer
	}
	bs := binary.LittleEndian.Uint32(s.hdr)
	if bs == eof {
		return 0, io.EOF
	}
	size = int(bs)
	return size, nil
}

func (s *stream) read(offset int) (n int, err error) {
	for n < offset {
		rn, re := s.conn.Read(s.buf[n:offset])
		if rn > 0 {
			n += rn
		}
		if re != nil {
			if re != io.EOF {
				return 0, re
			}
			err = re
			break
		}
		if n == offset {
			break
		}
	}
	return
}

func (s *stream) WriteTo(w io.Writer) (n int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buf = s.buf[:]
	s.hdr = s.hdr[:]

	for {
		if s.closed && s.sess.IsClosed() {
			return 0, ErrStreamClosed
		}

		size, erh := s.readHeader()
		if erh == io.EOF {
			break
		}

		rn, re := s.read(size)
		if re != nil {
			err = re
			break
		}

		if rn > 0 {
			p := s.buf[0:size]
			pl := len(p)
			wt := 0
			for wt < pl {
				nw, ew := w.Write(p)
				wt += nw
				if ew != nil {
					err = ew
					break
				}
			}
			if rn != wt {
				err = io.ErrShortWrite
			}
			n += int64(wt)
		}
		if err != nil {
			break
		}
	}
	return
}

func (s *stream) ReadFrom(r io.Reader) (n int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hdr = s.hdr[:]
	s.buf = s.buf[:]

	for {
		if s.closed && s.sess.IsClosed() {
			return 0, ErrStreamClosed
		}

		nr, er := r.Read(s.buf)
		if er != nil {
			if er == io.EOF || nr == 0 {
				err = s.writeEOF()
			}
			break
		}

		if whErr := s.writeHdr(nr); whErr != nil {
			return 0, whErr
		}

		if nr > 0 {
			nw, ew := s.conn.Write(s.buf[0:nr])
			if nw > 0 {
				n += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nw != nr {
				err = io.ErrShortWrite
				break
			}
		}
	}
	return
}

func (s *stream) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	s.sess.unregisterStream(s)
	_ = s.conn.Close()
	return nil
}
