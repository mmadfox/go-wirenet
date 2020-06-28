package wirenet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

const (
	initFrameTyp uint32 = 0x2
	errFrameTyp  uint32 = 0x4
	recvFrameTyp uint32 = 0x8
	permFrameTyp uint32 = 0x16

	openSessTyp  uint32 = 0x32
	confSessType uint32 = 0x64

	hdrLen       = 4
	headerLength = hdrLen * 3
)

type frame []byte

func (f frame) IsInitFrame() bool {
	return f.Type() == initFrameTyp
}

func (f frame) IsErrFrame() bool {
	return f.Type() == errFrameTyp
}

func (f frame) IsRecvFrame() bool {
	return f.Type() == recvFrameTyp
}

func (f frame) IsPermFrame() bool {
	return f.Type() == permFrameTyp
}

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

func sendFrame(name string, typ uint32, data []byte, rw io.ReadWriter) (frame, error) {
	if err := newEncoder(rw).Encode(typ, name, data); err != nil {
		return nil, err
	}
	frm, err := newDecoder(rw).Decode()
	if err != nil {
		return nil, err
	}
	if frm.IsErrFrame() || frm.IsPermFrame() {
		return nil, errors.New(string(frm.Payload()))
	}
	return frm, nil
}

func recvFrame(rw io.ReadWriter, fn func(frame) error) (frm frame, err error) {
	frm, err = newDecoder(rw).Decode()
	if err != nil {
		return nil, err
	}

	frameTyp := recvFrameTyp
	var frameErr []byte

	if fn != nil {
		if checkErr := fn(frm); checkErr != nil {
			frameTyp = errFrameTyp
			frameErr = []byte(checkErr.Error())
			err = checkErr
		}
	}

	if err := newEncoder(rw).Encode(frameTyp, frm.Command(), frameErr); err != nil {
		return nil, err
	}
	return frm, err
}
