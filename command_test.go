package wirenet

import (
	"bytes"
	"io"
	"testing"

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
