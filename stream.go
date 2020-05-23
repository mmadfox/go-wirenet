package wirenet

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"

	"github.com/google/uuid"
	"github.com/hashicorp/yamux"
)

const (
	bufSize = 1 << 10
	eof     = uint32(0)
)

type Stream interface {
	ID() uuid.UUID
	Session() Session
	Name() string
	IsClosed() bool
	Reader() io.ReadCloser
	Writer() io.WriteCloser
	io.Closer
	io.ReaderFrom
	io.WriterTo
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

func (s *stream) resetBuffers() {
	s.hdr = s.hdr[:]
	if len(s.buf) != bufSize {
		s.buf = make([]byte, bufSize)
	} else {
		s.buf = s.buf[:]
	}
}

func (s *stream) WriteTo(w io.Writer) (n int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resetBuffers()

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

	s.resetBuffers()

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

func (s *stream) Reader() io.ReadCloser {
	return &reader{
		stream: s,
	}
}

func (s *stream) Writer() io.WriteCloser {
	return &writer{
		stream: s,
	}
}

func (s *stream) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	s.sess.unregisterStream(s)
	_ = s.conn.Close()
	return nil
}

type writer struct {
	stream *stream
}

func (w *writer) Write(p []byte) (n int, err error) {
	if w.stream.IsClosed() && w.stream.sess.IsClosed() {
		return 0, ErrStreamClosed
	}
	if err := w.stream.writeHdr(len(p)); err != nil {
		return 0, err
	}
	return w.stream.conn.Write(p)
}

func (w *writer) Close() error {
	return w.stream.writeEOF()
}

type reader struct {
	stream *stream
	buf    *bytes.Buffer
	eof    bool
}

func (r *reader) fill() error {
	off, erh := r.stream.readHeader()
	if erh != nil {
		if erh == io.EOF {
			r.eof = true
		}
		return erh
	}

	b := make([]byte, off)
	n, err := r.read(b)
	if err != nil {
		return err
	}
	if n != off {
		return io.ErrShortBuffer
	}

	nw, erw := r.buf.Write(b)
	if erw != nil {
		return erw
	}
	if n != nw {
		return io.ErrShortWrite
	}
	return nil
}

func (r *reader) read(p []byte) (n int, err error) {
	for n < len(p) {
		rn, re := r.stream.conn.Read(p)
		if re != nil {
			if re != io.EOF {
				return 0, re
			}
			err = re
			break
		}
		if rn > 0 {
			n += rn
		}
		if n == len(p) {
			break
		}
	}
	return
}

func (r *reader) Close() error {
	if r.buf != nil {
		r.buf.Reset()
	}
	r.eof = false
	return nil
}

func (r *reader) Read(p []byte) (n int, err error) {
	if r.stream.IsClosed() && r.stream.sess.IsClosed() {
		return 0, ErrStreamClosed
	}
	if r.buf == nil {
		r.buf = new(bytes.Buffer)
	}
	total := r.buf.Len()
	for total < len(p) && !r.eof {
		if fer := r.fill(); fer != nil {
			break
		}
		total = r.buf.Len()
	}
	n, err = r.buf.Read(p)
	if n == 0 && r.eof {
		return 0, io.EOF
	}
	return
}
