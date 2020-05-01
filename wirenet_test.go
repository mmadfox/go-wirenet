package wirenet

import (
	"log"
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

func TestWire_OpenSession(t *testing.T) {
	t.Skip()
	addr := ":9087"
	wire, err := New(addr, ServerSide)
	assert.Nil(t, err)
	var totalSess int
	wire.CloseSession(func(s Session) error {
		totalSess--
		return nil
	})
	wire.OpenSession(func(s Session) error {
		totalSess++
		return nil
	})
	go func() {
		assert.Error(t, wire.Listen())
	}()
	time.Sleep(300 * time.Millisecond)
	for i := 0; i < 5; i++ {
		cliConn(t, addr, true)
		time.Sleep(300 * time.Millisecond)
	}
	assert.Equal(t, 0, totalSess)
}

func TestWire_Close(t *testing.T) {
	t.Skip()
	addr := ":9087"
	wire, err := New(addr, ServerSide)
	wire.OpenSession(func(s Session) error {
		log.Println("open session", s.ID())
		return nil
	})
	wire.CloseSession(func(s Session) error {
		log.Println("close session", s.ID())
		return nil
	})
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, wire.Listen())
	}()
	time.Sleep(100 * time.Millisecond)
	for i := 0; i < 5; i++ {
		cliConn(t, addr, false)
		time.Sleep(300 * time.Millisecond)
		if i > 1 {
			wire.Close()
		}
	}
	/*go func() {
		time.Sleep(5 * time.Second)
		wire.Close()
	}()*/
	time.Sleep(time.Minute)
}

func TestRole_String(t *testing.T) {
	assert.Equal(t, "client side wire", ClientSide.String())
	assert.Equal(t, "server side wire", ServerSide.String())
	assert.Equal(t, "unknown", Role(9).String())
}

func cliConn(t *testing.T, addr string, close bool) {
	conn, err := net.Dial("tcp", addr)
	log.Println("dial error", err)
	if close {
		defer conn.Close()
	}
	sess, err := yamux.Client(conn, nil)
	assert.Nil(t, err)
	assert.NotNil(t, sess)
}
