package wirenet

import (
	"io"
	"sync"

	"github.com/google/uuid"
)

const bufSize = 1 << 16

type Cmd interface {
	io.Closer
	io.ReaderFrom
	io.WriterTo
}

type command struct {
	id       uuid.UUID
	stream   io.ReadWriteCloser
	name     string
	isClosed bool
	buf      []byte
	cmdHub   commandHub
	lock     sync.RWMutex
}

func newCommand(name string, rwc io.ReadWriteCloser, cmdHub commandHub) *command {
	cmd := &command{
		id:     uuid.New(),
		name:   name,
		cmdHub: cmdHub,
		stream: rwc,
		buf:    make([]byte, bufSize),
	}
	cmdHub.Register(cmd)
	return cmd
}

func (c *command) ReadFrom(r io.Reader) (n int64, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.readFrom(r)
}

func (c *command) WriteTo(w io.Writer) (n int64, err error) {
	c.lock.Lock()
	c.lock.Unlock()
	return c.writeTo(w)
}

func (c *command) resetBuffer() {
	c.buf = c.buf[:]
}

func (c *command) readFrom(r io.Reader) (n int64, err error) {
	c.resetBuffer()
	for {
		if c.isClosed {
			return 0, ErrClosedCommand
		}
		nr, er := r.Read(c.buf)
		if nr > 0 {
			nw, ew := c.stream.Write(c.buf[0:nr])
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

func (c *command) writeTo(w io.Writer) (n int64, err error) {
	c.resetBuffer()
	for {
		if c.isClosed {
			return 0, ErrClosedCommand
		}
		nr, er := c.stream.Read(c.buf)
		if nr > 0 {
			nw, ew := w.Write(c.buf[0:nr])
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

func (c *command) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.isClosed {
		return ErrClosedCommand
	}

	c.isClosed = true
	c.cmdHub.Unregister(c)
	return c.stream.Close()
}
