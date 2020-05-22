package wirenet

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/assert"
)

func makeReadStream(addr string, t *testing.T) *stream {
	listener, err := net.Listen("tcp", addr)
	assert.Nil(t, err)

	conn, acceptErr := listener.Accept()
	assert.Nil(t, acceptErr)

	sess, err := yamux.Server(conn, nil)
	assert.Nil(t, err)

	sconn, err := sess.AcceptStream()
	assert.Nil(t, err)

	return &stream{
		conn: sconn,
		buf:  make([]byte, bufSize),
		hdr:  make([]byte, hdrLen),
	}
}

func makeWriteStream(addr string, t *testing.T) *stream {
	conn, err := net.Dial("tcp", addr)
	assert.Nil(t, err)

	sess, err := yamux.Client(conn, nil)
	assert.Nil(t, err)

	sconn, err := sess.OpenStream()
	assert.Nil(t, err)

	return &stream{
		conn: sconn,
		buf:  make([]byte, bufSize),
		hdr:  make([]byte, hdrLen),
	}
}

func TestStream_ReadFromWriteTo(t *testing.T) {
	addr := genAddr(t)
	done := make(chan struct{})
	want := 10000

	go func() {
		defer close(done)
		stream := makeReadStream(addr, t)
		// read
		buf := bytes.NewBuffer(nil)
		n, err := stream.WriteTo(buf)
		assert.Nil(t, err)
		assert.Equal(t, int(n), buf.Len())
		// write
		n, err = stream.ReadFrom(bytes.NewReader(genPayload(want)))
		assert.Nil(t, err)
		assert.Equal(t, want, int(n))

	}()

	time.Sleep(time.Second)

	// write
	stream := makeWriteStream(addr, t)
	n, err := stream.ReadFrom(bytes.NewReader(genPayload(want)))
	assert.Nil(t, err)
	assert.Equal(t, want, int(n))
	// read
	buf := bytes.NewBuffer(nil)
	n, err = stream.WriteTo(buf)
	assert.Nil(t, err)
	assert.Equal(t, int(n), buf.Len())

	<-done
}

func TestStream_ReadWrite(t *testing.T) {

}
