package wirenet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

const (
	initFrame uint32 = 0x2
	errFrame  uint32 = 0x4
	recvFrame uint32 = 0x6

	hdrLen       = 4
	headerLength = hdrLen * 3
)

type frame []byte

func (f frame) Type() uint32 {
	return binary.LittleEndian.Uint32(f[0:4])
}

func (f frame) CommandLen() int {
	return int(binary.LittleEndian.Uint32(f[4:8]))
}

func (f frame) PayloadLen() int {
	return int(binary.LittleEndian.Uint32(f[8:12]))
}

func (f frame) Command() string {
	return string(f[headerLength : len(f)-f.PayloadLen()])
}

func (f frame) Payload() []byte {
	return f[len(f)-f.PayloadLen():]
}

type encoder interface {
	Encode(uint32, string, []byte) error
}

type decoder interface {
	Decode() (frame, error)
}

type frameDecoder struct {
	r   io.Reader
	buf []byte
	hdr []byte
}

func (c *frameDecoder) Decode() (frame, error) {
	rn, err := c.r.Read(c.hdr)
	if err != nil {
		return nil, err
	}
	if rn != headerLength {
		return nil, io.ErrShortBuffer
	}

	var (
		cl  = int(binary.LittleEndian.Uint32(c.hdr[4:8]))
		pl  = int(binary.LittleEndian.Uint32(c.hdr[8:12]))
		off = int64(cl + pl)
	)
	if c.buf == nil {
		c.buf = make([]byte, off)
	}
	read := 0
	fl := int(off)
	for read < fl {
		rn, re := c.r.Read(c.buf[read:])
		if rn > 0 {
			read += rn
		}
		if read == fl || rn == 0 {
			break
		}
		if re != nil {
			if re != io.EOF {
				return nil, re
			}
			break
		}
	}
	return append(c.hdr, c.buf...), nil
}

func newDecoder(r io.Reader) decoder {
	return &frameDecoder{
		r:   r,
		hdr: make([]byte, headerLength),
	}
}

type frameEncoder struct {
	w   io.Writer
	buf *bytes.Buffer
}

func newEncoder(w io.Writer) encoder {
	return &frameEncoder{
		w:   w,
		buf: new(bytes.Buffer),
	}
}

func (c *frameEncoder) Encode(typ uint32, cmd string, payload []byte) error {
	var (
		cl = len(cmd)
		pl = len(payload)
		tl = int64(cl + pl + headerLength)
	)

	if err := binary.Write(c.buf, binary.LittleEndian, typ); err != nil {
		return err
	}
	if err := binary.Write(c.buf, binary.LittleEndian, uint32(cl)); err != nil {
		return err
	}
	if err := binary.Write(c.buf, binary.LittleEndian, uint32(pl)); err != nil {
		return err
	}

	c.buf.WriteString(cmd)
	c.buf.Write(payload)
	written, err := io.CopyN(c.w, c.buf, tl)
	if err != nil {
		return err
	}
	if written != tl {
		return io.ErrShortWrite
	}
	return nil
}

func sendInitFrame(name string, data []byte, rw io.ReadWriter) (frame, error) {
	if err := newEncoder(rw).Encode(initFrame, name, data); err != nil {
		return nil, err
	}
	frm, err := newDecoder(rw).Decode()
	if err != nil {
		return nil, err
	}
	if frm.Type() == errFrame {
		return nil, errors.New(string(frm.Payload()))
	}
	return frm, nil
}

func recvInitFrame(rw io.ReadWriter, check func(frame) error) (frame, error) {
	frm, err := newDecoder(rw).Decode()
	if err != nil {
		return nil, err
	}
	ft := recvFrame
	var errData []byte
	if err := check(frm); err != nil {
		ft = errFrame
		errData = []byte(err.Error())
	}
	if err := newEncoder(rw).Encode(ft, frm.Command()+"-reply", errData); err != nil {
		return nil, err
	}
	return frm, nil
}
