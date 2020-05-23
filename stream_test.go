package wirenet

import (
	"bytes"
	"encoding/json"
	"net"
	"sync"
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
		buf:  make([]byte, BufSize),
		hdr:  make([]byte, hdrLen),
		sess: new(session),
		mu:   sync.RWMutex{},
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
		buf:  make([]byte, BufSize),
		hdr:  make([]byte, hdrLen),
		sess: new(session),
		mu:   sync.RWMutex{},
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

func TestStream_ReaderWriter(t *testing.T) {
	addr := genAddr(t)
	done := make(chan struct{})
	want := 10
	payload := genPayload(want)
	iter := 4

	go func() {
		defer close(done)
		stream := makeReadStream(addr, t)
		reader := stream.Reader()
		read := 0
		buf := make([]byte, 1000)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				break
			}
			read += n
		}
		assert.Equal(t, len(payload)*iter, read)
		assert.Nil(t, reader.Close())

		for {
			n, err := reader.Read(buf)
			if err != nil {
				break
			}
			read += n
		}
		assert.Equal(t, len(payload)*iter*2, read)
		assert.Nil(t, reader.Close())
	}()

	time.Sleep(time.Second)

	// write
	stream := makeWriteStream(addr, t)
	writer := stream.Writer()
	for i := 0; i < iter; i++ {
		n, err := writer.Write(payload)
		assert.Nil(t, err)
		assert.Equal(t, len(payload), n)
	}
	assert.Nil(t, writer.Close())

	for i := 0; i < 4; i++ {
		n, err := writer.Write(payload)
		assert.Nil(t, err)
		assert.Equal(t, len(payload), n)
	}
	assert.Nil(t, writer.Close())

	<-done
}

func TestStream_ReaderWriterJSON(t *testing.T) {
	addr := genAddr(t)
	done := make(chan struct{})
	want := "HELLO"

	go func() {
		defer close(done)
		stream := makeReadStream(addr, t)
		reader := stream.Reader()
		var have string
		err := json.NewDecoder(reader).Decode(&have)
		assert.Nil(t, err)
		assert.Equal(t, want, have)
		assert.Nil(t, reader.Close())
	}()

	time.Sleep(time.Second)

	// write
	stream := makeWriteStream(addr, t)
	writer := stream.Writer()
	err := json.NewEncoder(writer).Encode("HELLO")
	assert.Nil(t, err)
	assert.Nil(t, writer.Close())

	<-done
}

func TestStream_ReadWriteTo(t *testing.T) {
	addr := genAddr(t)
	done := make(chan struct{})
	want := "HELLO"

	go func() {
		defer close(done)
		stream := makeReadStream(addr, t)
		reader := stream.Reader()
		read := 0
		buf := make([]byte, 1)
		res := bytes.NewBuffer(nil)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				break
			}
			res.Write(buf[:n])
			read += n
		}
		assert.Equal(t, len(want), read)
		assert.Equal(t, want, res.String())
		assert.Nil(t, reader.Close())
	}()

	time.Sleep(time.Second)

	// write
	stream := makeWriteStream(addr, t)
	n, err := stream.ReadFrom(bytes.NewReader([]byte(want)))
	assert.Nil(t, err)
	assert.Equal(t, len(want), int(n))

	<-done
}
