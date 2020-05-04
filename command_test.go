package wirenet

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"
)

type nopCloser struct {
	io.ReadWriter
}

func NopCloser(rw io.ReadWriter) io.ReadWriteCloser {
	return nopCloser{rw}
}

func (c nopCloser) Close() error {
	return nil
}

type swr struct {
	errTyp int
	n      int
}

func (s swr) Read(p []byte) (n int, err error) {
	if s.errTyp == 1 {
		return s.n, errors.New("read error")
	}
	if s.errTyp == 2 {
		return s.n, nil
	}
	if s.errTyp == 3 {
		return s.n, nil
	}
	return
}

func (s swr) Write(p []byte) (n int, err error) {
	if s.errTyp == 1 {
		return s.n, errors.New("written error")
	}
	if s.errTyp == 2 {
		return s.n, nil
	}
	if s.errTyp == 3 {
		return s.n, nil
	}
	return
}

func (s swr) Close() error {
	return nil
}

func TestCommand_ReadFrom(t *testing.T) {
	ctrl := gomock.NewController(t)
	hub := NewMockcommandHub(ctrl)
	hub.EXPECT().Register(gomock.Any())
	hub.EXPECT().Unregister(gomock.Any())
	stream := NopCloser(bytes.NewBuffer(nil))
	cmd := newCommand("readFrom", stream, hub)
	assert.NotNil(t, cmd)
	payload := genPayload(2000000)
	n, err := cmd.ReadFrom(bytes.NewReader(payload))
	assert.Nil(t, err)
	assert.Equal(t, int64(len(payload)), n)
}

func TestCommand_ReadFromWriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	hub := NewMockcommandHub(ctrl)
	hub.EXPECT().Register(gomock.Any())
	hub.EXPECT().Unregister(gomock.Any())
	cmd := newCommand("readFrom", swr{errTyp: 1, n: 300}, hub)
	assert.NotNil(t, cmd)
	payload := genPayload(2000000)
	n, err := cmd.ReadFrom(bytes.NewReader(payload))
	assert.Equal(t, int64(300), n)
	assert.EqualError(t, err, "written error")
}

func TestCommand_ReadFromWriteShortWriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	hub := NewMockcommandHub(ctrl)
	hub.EXPECT().Register(gomock.Any())
	hub.EXPECT().Unregister(gomock.Any())
	cmd := newCommand("readFrom", swr{errTyp: 2, n: 900}, hub)
	assert.NotNil(t, cmd)
	payload := genPayload(2000000)
	n, err := cmd.ReadFrom(bytes.NewReader(payload))
	assert.Equal(t, int64(900), n)
	assert.Equal(t, err, io.ErrShortWrite)
}

func TestCommand_WriteToWriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	hub := NewMockcommandHub(ctrl)
	hub.EXPECT().Register(gomock.Any())
	hub.EXPECT().Unregister(gomock.Any())
	cmd := newCommand("writeTo", swr{errTyp: 1, n: 300}, hub)
	assert.NotNil(t, cmd)
	n, err := cmd.WriteTo(bytes.NewBuffer(nil))
	assert.Equal(t, int64(300), n)
	assert.EqualError(t, err, "read error")
}

func TestCommand_WriteToShortWriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	hub := NewMockcommandHub(ctrl)
	hub.EXPECT().Register(gomock.Any())
	hub.EXPECT().Unregister(gomock.Any())
	cmd := newCommand("writeTo", swr{errTyp: 3, n: 900}, hub)
	assert.NotNil(t, cmd)
	n, err := cmd.WriteTo(swr{errTyp: 2, n: 910})
	assert.Equal(t, int64(910), n)
	assert.Equal(t, err, io.ErrShortWrite)
}

func TestCommand_WriteToError(t *testing.T) {
	ctrl := gomock.NewController(t)
	hub := NewMockcommandHub(ctrl)
	hub.EXPECT().Register(gomock.Any())
	hub.EXPECT().Unregister(gomock.Any())
	cmd := newCommand("writeTo", swr{errTyp: 3, n: 900}, hub)
	assert.NotNil(t, cmd)
	n, err := cmd.WriteTo(swr{errTyp: 1, n: 910})
	assert.Equal(t, int64(910), n)
	assert.EqualError(t, err, "written error")
}

func TestCommand_WriteTo(t *testing.T) {
	ctrl := gomock.NewController(t)
	hub := NewMockcommandHub(ctrl)
	hub.EXPECT().Register(gomock.Any())
	hub.EXPECT().Unregister(gomock.Any())
	payload := genPayload(2000000)
	stream := NopCloser(bytes.NewBuffer(payload))
	cmd := newCommand("writeTo", stream, hub)
	assert.NotNil(t, cmd)
	writer := bytes.NewBuffer(nil)
	n, err := cmd.WriteTo(writer)
	assert.Nil(t, err)
	assert.Equal(t, int64(len(payload)), n)
}

func TestCommand_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	hub := NewMockcommandHub(ctrl)
	hub.EXPECT().Register(gomock.Any())
	hub.EXPECT().Unregister(gomock.Any()).Do(func(cmd *command) {
		assert.NotEmpty(t, cmd.id)
	})
	stream := NopCloser(bytes.NewBuffer(nil))
	cmd := newCommand("writeTo", stream, hub)
	assert.NotNil(t, cmd)
	go func() {
		time.Sleep(10 * time.Millisecond)
		err := cmd.Close()
		assert.Nil(t, err)
		err = cmd.Close()
		assert.Equal(t, ErrClosedCommand, err)
	}()
	n, err := cmd.ReadFrom(bytes.NewReader(genPayload(99999999)))
	assert.Equal(t, ErrClosedCommand, err)
	assert.Empty(t, n)
	n, err = cmd.WriteTo(bytes.NewBuffer(nil))
	assert.Empty(t, n)
	assert.Equal(t, ErrClosedCommand, err)
}
