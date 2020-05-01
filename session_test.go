package wirenet

import (
	"log"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/stretchr/testify/assert"
)

func TestSession_Close(t *testing.T) {
	t.Skip()

	w := &wire{
		regSessCh:   make(chan *session),
		unRegSessCh: make(chan *session),
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
			time.Sleep(1 * time.Second)
			err = sess.Close()
			assert.Nil(t, err)
			log.Println("close session err", err)
			return
		}
	}()

	time.Sleep(500 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	assert.Nil(t, err)
	sess, err := yamux.Client(conn, nil)
	assert.Nil(t, err)
	for i := 0; i < 5; i++ {
		sess.OpenStream()
		time.Sleep(500 * time.Millisecond)
	}

	time.Sleep(3 * time.Second)
}
