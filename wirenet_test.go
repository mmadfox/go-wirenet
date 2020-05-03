package wirenet

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	wire, err := New(":8080", Role(5))
	assert.Equal(t, ErrUnknownListenerSide, err)
	assert.Nil(t, wire)

	wire, err = New("", ClientSide)
	assert.Equal(t, ErrListenerAddrEmpty, err)
	assert.Nil(t, wire)

	wire, err = New(":9090", ClientSide,
		WithKeepAlive(true),
		WithKeepAliveInterval(DefaultKeepAliveInterval),
		WithReadWriteTimeouts(DefaultWriteTimeout, DefaultWriteTimeout),
	)
	assert.Nil(t, err)
	assert.NotNil(t, wire)
}

func TestWire_ListenClientSide(t *testing.T) {
	t.Skip()
	addr := ":9090"
	go func() {
		wireSrv, err := New(addr, ServerSide)
		assert.Nil(t, err)
		go func() {
			time.Sleep(5 * time.Second)
			assert.Nil(t, wireSrv.Close())
		}()
		assert.Nil(t, wireSrv.Listen())

	}()
	time.Sleep(time.Second)
	wire, err := New(addr, ClientSide)
	assert.Nil(t, err)
	assert.Nil(t, wire.Listen())
	time.Sleep(time.Hour)
}

func TestWire_Close(t *testing.T) {
	t.Skip()
	port, err := RandomPort()
	assert.Nil(t, err)
	addr := fmt.Sprintf(":%d", port)
	wire, err := New(addr, ServerSide)
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, wire.Listen())
	}()
	time.Sleep(time.Second)
	for sn := 0; sn < 10; sn++ {
		go func(id int) {
			sess := makeConn(t, addr)
			for x := 0; x < 10; x++ {
				conn, err := sess.OpenStream()
				if err != nil {
					return
				}
				func(stream *yamux.Stream, sid int) {
					defer stream.Close()
					time.Sleep(15 * time.Second)
				}(conn, id)
			}
		}(sn)
	}
	errCh := make(chan error)
	go func() {
		time.Sleep(2 * time.Second)
		errCh <- wire.Close()
		return
	}()
	assert.Nil(t, <-errCh)
}

func TestRole_String(t *testing.T) {
	assert.Equal(t, "client side wire", ClientSide.String())
	assert.Equal(t, "server side wire", ServerSide.String())
	assert.Equal(t, "unknown", Role(9).String())
}

func makeConn(t *testing.T, addr string) *yamux.Session {
	conn, err := net.Dial("tcp", addr)
	assert.Nil(t, err)
	sess, err := yamux.Client(conn, nil)
	assert.Nil(t, err)
	assert.NotNil(t, sess)
	return sess
}
