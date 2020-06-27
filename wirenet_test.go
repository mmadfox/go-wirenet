package wirenet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func genAddr(t *testing.T) string {
	if t == nil {
		t = new(testing.T)
	}
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	assert.Nil(t, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.Nil(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf(":%d", port)
}

type balance struct {
	Amount int
	OpName string
}

func server(addr string, t *testing.T) (closer chan io.Closer) {
	closer = make(chan io.Closer)
	go func() {
		srv, err := Mount(addr,
			WithConnectHook(func(c io.Closer) {
				closer <- c
				close(closer)
			}))
		assert.Nil(t, err)
		srv.Stream("server:readBalance", func(ctx context.Context, stream Stream) {
			writer := stream.Writer()
			defer func() {
				assert.Nil(t, writer.Close())
			}()
			err := json.NewEncoder(writer).Encode(balance{
				Amount: 100,
				OpName: "debit",
			})
			assert.Nil(t, err)
		})
		err = srv.Connect()
		assert.Nil(t, err)
		assert.Empty(t, srv.Sessions())
	}()
	return closer
}

func client(addr string, wg *sync.WaitGroup, t *testing.T) {
	go func() {
		cli, err := Join(addr, WithSessionOpenHook(func(s Session) {
			stream, err := s.OpenStream("server:readBalance")
			assert.Nil(t, err)
			defer stream.Close()
			var b balance
			reader := stream.Reader()
			reader.Close()
			assert.Nil(t, json.NewDecoder(reader).Decode(&b))
			assert.Equal(t, 100, b.Amount)
			wg.Done()
		}))
		assert.Nil(t, err)
		assert.Nil(t, cli.Connect())
	}()
}

func TestWire_New(t *testing.T) {
	wire, err := Mount("")
	assert.Nil(t, wire)
	assert.Equal(t, ErrAddrEmpty, err)
}

func TestWire_Close(t *testing.T) {
	addr := genAddr(t)
	maxSess := 3
	srv := <-server(addr, t)
	var wg sync.WaitGroup
	for i := 0; i < maxSess; i++ {
		wg.Add(1)
		client(addr, &wg, t)
	}
	wg.Wait()
	srv.Close()
}

func TestWire_OpenCloseSession(t *testing.T) {
	addr := genAddr(t)
	var wireSrv Wire
	listen := make(chan struct{})

	var openSessCounter int32
	var closeSessCounter int32
	maxSess := int32(5)

	// server
	go func() {
		srv, err := Mount(addr,
			WithConnectHook(func(_ io.Closer) { close(listen) }),
			WithSessionOpenHook(func(s Session) {
				atomic.AddInt32(&openSessCounter, 1)
			}),
			WithSessionCloseHook(func(s Session) {
				atomic.AddInt32(&closeSessCounter, 1)
			}))
		assert.Nil(t, err)
		wireSrv = srv
		srv.Connect()
	}()
	<-listen
	// client
	var wg sync.WaitGroup
	for i := int32(0); i < maxSess; i++ {
		wg.Add(1)
		go func() {
			open := make(chan struct{})
			wireCli, err := Join(addr, WithSessionOpenHook(func(s Session) {
				close(open)
			}))
			go func() {
				<-open
				<-time.After(time.Second)
				assert.Nil(t, wireCli.Close())
				time.Sleep(500 * time.Millisecond)
				wg.Done()
			}()
			assert.Nil(t, err)
			assert.Nil(t, wireCli.Connect())
		}()
	}
	wg.Wait()
	assert.Equal(t, maxSess, atomic.LoadInt32(&closeSessCounter), "closeCounter")
	assert.Equal(t, maxSess, atomic.LoadInt32(&openSessCounter), "openCounter")
	assert.Nil(t, wireSrv.Close())
}

func TestWire_ListenServer(t *testing.T) {
	addr := genAddr(t)
	wire, err := Mount(addr, WithConnectHook(func(w io.Closer) {
		assert.Nil(t, w.Close())
	}))
	assert.Nil(t, err)
	assert.Nil(t, wire.Connect())
}

func TestWire_ListenClient(t *testing.T) {
	addr := genAddr(t)
	var wireSrv Wire

	listen := make(chan struct{})
	client := make(chan struct{})
	var conn int32

	// server
	go func() {
		srv, err := Mount(addr,
			WithSessionOpenHook(func(s Session) {
				atomic.AddInt32(&conn, 1)
			}),
			WithConnectHook(func(_ io.Closer) {
				close(listen)
			}))
		assert.Nil(t, err)
		wireSrv = srv
		wireSrv.Connect()
	}()
	<-listen

	go func() {
		// client
		wireCli, err := Join(addr, WithSessionOpenHook(func(s Session) {
			atomic.AddInt32(&conn, 1)
			s.Close()
			close(client)
		}))
		assert.Nil(t, err)
		assert.Nil(t, wireCli.Connect())
	}()
	<-client

	assert.Equal(t, atomic.LoadInt32(&conn), int32(2))
	wireSrv.Close()
}
