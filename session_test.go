package wirenet

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/stretchr/testify/assert"
)

func TestSession_Command(t *testing.T) {
	port, err := RandomPort()
	assert.Nil(t, err)
	addr := fmt.Sprintf(":%d", port)

	go func() {
		wire, err := New(addr, ServerSide)
		assert.Nil(t, err)
		wire.VerifyToken(func(cmd string, token []byte) error {
			return nil
		})
		assert.Nil(t, wire.Listen())
	}()

	time.Sleep(time.Second)

	wire, err := New(addr, ClientSide)
	assert.Nil(t, err)
	wire.WithToken([]byte("token"))
	wire.OpenSession(func(s Session) error {
		cmd, err := s.Command("ls")
		assert.Nil(t, err)
		cmd = cmd
		return nil
	})
	wire.CloseSession(func(s Session) error {
		return nil
	})
	assert.Nil(t, wire.Listen())
}

func TestSession_Close(t *testing.T) {
	t.Skip()

	w := &wire{
		sessCloseTimeout: 5 * time.Second,
	}

	addr := ":4545"

	go func() {
		lis, err := net.Listen("tcp", addr)
		assert.Nil(t, err)
		for {
			conn, err := lis.Accept()
			assert.Nil(t, err)
			if err != nil {
				return
			}
			srv, err := yamux.Server(conn, nil)
			assert.Nil(t, err)
			sess := newSession(w, conn, srv)
			assert.Nil(t, err)
			go sess.handle()

			go func() {
				time.Sleep(10 * time.Second)
				err = sess.Close()
				assert.Nil(t, err)
				return
			}()
		}
	}()

	time.Sleep(500 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	assert.Nil(t, err)
	sess, err := yamux.Client(conn, nil)
	assert.Nil(t, err)
	for i := 0; i < 5; i++ {
		sess.OpenStream()
		time.Sleep(1 * time.Second)
	}

	time.Sleep(60 * time.Second)
}
