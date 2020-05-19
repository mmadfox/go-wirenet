package wirenet

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

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

func TestWire_New(t *testing.T) {
	addr := genAddr(t)
	wire, err := New(addr, Role(999))
	assert.Nil(t, wire)
	assert.Equal(t, ErrUnknownListenerSide, err)

	wire, err = New("", ServerSide)
	assert.Nil(t, wire)
	assert.Equal(t, ErrAddrEmpty, err)
}

func TestWire_StreamClientToServerSomeData(t *testing.T) {
	addr := genAddr(t)
	listen := make(chan struct{})
	var wireSrv Wire
	go func() {
		srv, err := NewServer(addr, WithConnectHook(func(_ io.Closer) {
			close(listen)
		}))
		assert.Nil(t, err)
		srv.Mount("ls", func(ls Stream) {
			buf := make([]byte, 32)
			for i := uint32(0); i < 100; i++ {
				binary.LittleEndian.PutUint32(buf, i)
				n, err := ls.Write(buf)
				assert.Nil(t, err)
				assert.Equal(t, len(buf), n)
				time.Sleep(10 * time.Millisecond)
			}
		})
		wireSrv = srv
		srv.Connect()
	}()
	<-listen

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			init := make(chan struct{})
			cli, err := NewClient(addr, WithOpenSessionHook(func(s Session) {
				close(init)
			}))
			go func() {
				<-init
				defer wg.Done()
				ls, err := cli.Stream("ls")
				assert.Nil(t, err)
				buf := make([]byte, 32)
				total := 0
				var rn uint32
				for {
					n, err := ls.Read(buf)
					if err != nil {
						break
					}
					total += n
					rn += binary.LittleEndian.Uint32(buf)
				}
				assert.Nil(t, cli.Close())
			}()
			assert.Nil(t, err)
			assert.Nil(t, cli.Connect())
		}()
	}
	wg.Wait()
	assert.Nil(t, wireSrv.Close())
}

func TestWire_StreamServerToClient(t *testing.T) {
	addr := genAddr(t)
	listen := make(chan struct{})
	workerNum := 5
	want := 5 * 7
	var have int
	counter := make(chan int, workerNum)

	// server side
	go func() {
		sessions := make(chan uuid.UUID)
		srv, err := NewServer(addr,
			WithOpenSessionHook(func(s Session) {
				sessions <- s.ID()
			}),
			WithConnectHook(func(closer io.Closer) {
				close(listen)
			}))
		assert.Nil(t, err)
		go func() {
			var sessCount int
			for sessCount < workerNum {
				select {
				case <-sessions:
					sessCount++
				}
			}
			for c := 0; c < sessCount; c++ {
				st, err := srv.Stream(fmt.Sprintf("host%d:cat", c))
				assert.Nil(t, err)
				payload := fmt.Sprintf("string%d", c)
				n, err := st.Write([]byte(payload))
				assert.Nil(t, err)
				assert.Equal(t, len(payload), n)
			}
		}()
		assert.Nil(t, srv.Connect())
	}()
	<-listen

	var wg sync.WaitGroup
	for i := 0; i < workerNum; i++ {
		wg.Add(1)
		go func(n int) {
			cli, err := NewClient(addr)
			assert.Nil(t, err)
			cli.Mount(fmt.Sprintf("host%d:cat", n), func(s Stream) {
				str := make([]byte, 7)
				n, err := s.Read(str)
				assert.Nil(t, err)
				assert.NotEmpty(t, n)
				assert.Contains(t, string(str), "string")
				counter <- n
				wg.Done()
			})
			assert.Nil(t, cli.Connect())
		}(i)
	}
	wg.Wait()

	for i := 0; i < workerNum; i++ {
		have += <-counter
	}
	assert.Equal(t, want, have)
}

func TestWire_StreamClientToServerJSON(t *testing.T) {
	addr := genAddr(t)
	listen := make(chan struct{})
	var wireSrv Wire
	type want struct {
		Name string
		Age  int
	}
	go func() {
		srv, err := NewServer(addr, WithConnectHook(func(_ io.Closer) {
			close(listen)
		}))
		assert.Nil(t, err)
		srv.Mount("ls", func(ls Stream) {
			// doing...
			err := json.NewEncoder(ls).Encode(want{Name: "name", Age: 888})
			assert.Nil(t, err)
		})
		wireSrv = srv
		srv.Connect()
	}()
	<-listen

	init := make(chan struct{})
	cli, err := NewClient(addr, WithOpenSessionHook(func(s Session) {
		close(init)
	}))
	go func() {
		<-init
		ls, err := cli.Stream("ls")
		assert.Nil(t, err)
		var res want
		err = json.NewDecoder(ls).Decode(&res)
		assert.Nil(t, err)
		assert.Equal(t, "name", res.Name)
		assert.Equal(t, 888, res.Age)
		assert.Nil(t, cli.Close())
	}()
	assert.Nil(t, err)
	assert.Nil(t, cli.Connect())
	assert.Nil(t, wireSrv.Close())
}

func TestWire_StreamClientToServer(t *testing.T) {
	addr := genAddr(t)
	listen := make(chan struct{})
	var wireSrv Wire
	want := []byte("wirenet")
	go func() {
		srv, err := NewServer(addr, WithConnectHook(func(_ io.Closer) {
			close(listen)
		}))
		assert.Nil(t, err)
		srv.Mount("ls", func(ls Stream) {
			n, err := ls.ReadFrom(bytes.NewReader(want))
			assert.Nil(t, err)
			assert.Equal(t, int64(len(want)), n)
		})
		wireSrv = srv
		srv.Connect()
	}()
	<-listen

	init := make(chan struct{})
	cli, err := NewClient(addr, WithOpenSessionHook(func(s Session) {
		close(init)
	}))
	go func() {
		<-init
		ls, err := cli.Stream("ls")
		assert.Nil(t, err)
		buf := bytes.NewBuffer(nil)
		n, err := ls.WriteTo(buf)
		assert.Nil(t, err)
		assert.Equal(t, int64(len(want)), n)
		assert.Nil(t, cli.Close())
	}()
	assert.Nil(t, err)
	assert.Nil(t, cli.Connect())
	assert.Nil(t, wireSrv.Close())
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
		srv, err := NewServer(addr,
			WithConnectHook(func(_ io.Closer) { close(listen) }),
			WithOpenSessionHook(func(s Session) {
				atomic.AddInt32(&openSessCounter, 1)
			}),
			WithCloseSessionHook(func(s Session) {
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
			wireCli, err := NewClient(addr, WithOpenSessionHook(func(s Session) {
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
	wire, err := NewServer(addr, WithConnectHook(func(w io.Closer) {
		assert.Nil(t, w.Close())
	}))
	assert.Nil(t, err)
	assert.Nil(t, wire.Connect())
}

func TestWire_ListenClient(t *testing.T) {
	addr := genAddr(t)
	var wireSrv Wire
	listen := make(chan struct{})
	// server
	go func() {
		srv, err := NewServer(addr,
			WithConnectHook(func(_ io.Closer) {
				close(listen)
			}))
		assert.Nil(t, err)
		wireSrv = srv
		wireSrv.Connect()
	}()
	<-listen
	// client
	wireCli, err := NewClient(addr, WithConnectHook(func(w io.Closer) {
		assert.Nil(t, w.Close())
		assert.Equal(t, ErrWireClosed, w.Close())
	}))
	assert.Nil(t, err)
	assert.Nil(t, wireCli.Connect())
	assert.Nil(t, wireSrv.Close())
	assert.Equal(t, ErrWireClosed, wireSrv.Close())
}

func BenchmarkWire_ListenServer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		addr := ":4578"
		wire, err := NewServer(addr,
			WithConnectHook(func(w io.Closer) {
				_ = w.Close()
			}))
		if err != nil {
			b.Fatal(err)
		}
		if err := wire.Connect(); err != nil {
			b.Fatal(err)
		}
	}
}
