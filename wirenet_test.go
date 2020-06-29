package wirenet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
)

func TestRole_IsClientSide(t *testing.T) {
	assert.Equal(t, clientSide, role(1))
}

func TestRole_IsServerSide(t *testing.T) {
	assert.Equal(t, serverSide, role(2))
}

func TestRole_String(t *testing.T) {
	assert.Equal(t, clientSide.String(), "client side")
	assert.Equal(t, serverSide.String(), "server side")
	assert.Equal(t, role(999).String(), "unknown")
}

func TestHub(t *testing.T) {
	w, err := Hub(":8989", nil)
	assert.Nil(t, err)
	assert.True(t, w.(*wire).hubMode)
}

func TestJoin(t *testing.T) {
	w, err := Join(":8989", nil)
	assert.Nil(t, err)
	assert.True(t, w.(*wire).role.IsClientSide())
}

func TestMount(t *testing.T) {
	w, err := Mount("")
	assert.Nil(t, w)
	assert.Equal(t, ErrAddrEmpty, err)

	w, err = Mount(":8989", nil)
	assert.Nil(t, err)
	assert.True(t, w.(*wire).role.IsServerSide())
}

func TestWire_Connect(t *testing.T) {
	addr := genAddr(t)
	conn := make(chan struct{})

	// server side
	server, err := Mount(addr, WithConnectHook(func(closer io.Closer) {
		close(conn)
	}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-conn

	client, err := Join(addr, WithConnectHook(func(closer io.Closer) {
		time.Sleep(300 * time.Millisecond)
		assert.Nil(t, closer.Close())
		assert.Nil(t, server.Close())
	}))
	assert.Nil(t, err)
	assert.Nil(t, client.Connect())
}

func TestWire_ConnectDup(t *testing.T) {
	addr := genAddr(t)
	conn := make(chan struct{})
	// server side
	server, err := Mount(addr, WithConnectHook(func(closer io.Closer) {
		close(conn)
	}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-conn

	server, err = Mount(addr)
	assert.Nil(t, err)
	assert.Error(t, server.Connect())
}

func TestWire_ConnectHub(t *testing.T) {
	addr := genAddr(t)
	initHub := make(chan struct{})

	srvToken := Token("token")
	tokenErr := errors.New("token invalid")

	timeout := time.Hour
	payload1 := []byte("client1")
	payload2 := []byte("client2")

	// hub
	serverTLSConf, err := LoadCertificates("server", "./certs")
	assert.Nil(t, err)
	hub, err := Hub(addr,
		WithTLS(serverTLSConf),
		WithReadWriteTimeouts(timeout, timeout),
		WithSessionCloseTimeout(time.Second),
		WithTokenValidator(func(streamName string, id Identification, token Token) error {
			if bytes.Equal(srvToken, token) {
				return nil
			}
			return tokenErr
		}),
		WithConnectHook(func(closer io.Closer) {
			close(initHub)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, hub.Connect())
	}()
	<-initHub

	var wg sync.WaitGroup
	wg.Add(2)

	var sess1 Session
	var sess2 Session

	// clients
	clientTLSConf, err := LoadCertificates("client", "./certs")
	assert.Nil(t, err)
	clientTLSConf.InsecureSkipVerify = true

	// client one
	id1 := Identification("client1")
	client1, err := Join(addr,
		WithSessionOpenHook(func(s Session) {
			sess1 = s
			wg.Done()
		}),
		WithIdentification(id1, srvToken),
		WithTLS(clientTLSConf),
	)
	assert.Nil(t, err)
	client1.Stream("c1:codec", func(ctx context.Context, s Stream) {
		w := s.Writer()
		w.Write(payload1)
		w.Close()
	})
	go func() {
		assert.Nil(t, client1.Connect())
	}()

	// client two
	id2 := Identification("client2")
	client2, err := Join(addr,
		WithSessionOpenHook(func(s Session) {
			sess2 = s
			wg.Done()
		}),
		WithIdentification(id2, srvToken),
		WithTLS(clientTLSConf),
	)
	assert.Nil(t, err)
	client2.Stream("c2:codec", func(ctx context.Context, s Stream) {
		w := s.Writer()
		w.Write(payload2)
		w.Close()
	})
	go func() {
		assert.Nil(t, client2.Connect())
	}()
	wg.Wait()

	buf := bytes.NewBuffer(nil)

	// client1 -> client2
	s, err := sess1.OpenStream("c2:codec")
	assert.Nil(t, err)
	n, err := s.WriteTo(buf)
	assert.Nil(t, err)
	assert.Equal(t, int64(len(payload2)), n)
	assert.Nil(t, s.Close())

	// client2 -> client1
	s, err = sess2.OpenStream("c1:codec")
	assert.Nil(t, err)
	n, err = s.WriteTo(buf)
	assert.Nil(t, err)
	assert.Equal(t, int64(len(payload1)), n)
	assert.Nil(t, s.Close())

	assert.Equal(t, "client2client1", buf.String())
	assert.Nil(t, client1.Close())
	assert.Nil(t, client2.Close())
	assert.Nil(t, hub.Close())
}

func TestWire_ConnectTLS(t *testing.T) {
	addr := genAddr(t)
	conn := make(chan struct{})

	// server side
	serverTLSConf, err := LoadCertificates("server", "./certs")
	assert.Nil(t, err)
	server, err := Mount(addr,
		WithTLS(serverTLSConf),
		WithConnectHook(func(closer io.Closer) {
			close(conn)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-conn

	clientTLSConf, err := LoadCertificates("client", "./certs")
	assert.Nil(t, err)
	clientTLSConf.InsecureSkipVerify = true
	client, err := Join(addr,
		WithTLS(clientTLSConf),
		WithConnectHook(func(closer io.Closer) {
			time.Sleep(time.Second)
			assert.Nil(t, closer.Close())
			assert.Nil(t, server.Close())
		}))
	assert.Nil(t, err)
	assert.Nil(t, client.Connect())
}

func TestWire_ReConnect(t *testing.T) {
	addr := genAddr(t)
	retryMin := 1 * time.Second
	retryMax := 2 * time.Second
	retryNum := 2
	var retryCounter int
	client, err := Join(addr,
		WithRetryPolicy(func(min, max time.Duration, attemptNum int) time.Duration {
			retryCounter++
			return DefaultRetryPolicy(min, max, attemptNum)
		}),
		WithRetryMax(retryNum),
		WithRetryWait(retryMin, retryMax),
	)
	assert.Nil(t, err)
	assert.Contains(t, client.Connect().Error(), "connection refused")
	assert.Equal(t, retryNum, retryCounter)
}

func TestWire_Session(t *testing.T) {
	addr := genAddr(t)
	initSrv := make(chan struct{})
	initCli := make(chan struct{})

	// server side
	server, err := Mount(addr, WithConnectHook(func(closer io.Closer) {
		close(initSrv)
	}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	client, err := Join(addr, WithConnectHook(func(closer io.Closer) {
		time.Sleep(time.Second)
		close(initCli)
	}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, client.Connect())
	}()
	<-initCli

	s, err := server.Session(uuid.New())
	assert.Error(t, ErrSessionNotFound, err)
	assert.Nil(t, s)

	for _, sess := range server.Sessions() {
		found, err := client.Session(sess.ID())
		assert.Nil(t, err)
		assert.Equal(t, found.ID(), sess.ID())
	}

	assert.Nil(t, client.Close())
	assert.Nil(t, server.Close())
}

func TestWire_ErrorHandler(t *testing.T) {
	addr := genAddr(t)
	initSrv := make(chan struct{})
	initCli := make(chan Session)

	// server side
	server, err := Mount(addr,
		WithErrorHandler(func(_ context.Context, err error) {
			assert.Contains(t, err.Error(), "validate stream")
		}),
		WithConnectHook(func(closer io.Closer) {
			time.Sleep(time.Second)
			close(initSrv)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	client, err := Join(addr, WithSessionOpenHook(func(s Session) {
		initCli <- s
	}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, client.Connect())
	}()
	sess := <-initCli

	stream, err := sess.OpenStream("unknown")
	assert.Nil(t, stream)
	assert.Equal(t, ErrStreamHandlerNotFound, err)
}

func TestWire_AuthFailed(t *testing.T) {
	addr := genAddr(t)
	initSrv := make(chan struct{})
	initCli := make(chan Session)

	tokenErr := errors.New("token invalid")

	// server side
	server, err := Mount(addr,
		WithTokenValidator(func(streamName string, id Identification, token Token) error {
			if streamName == "confirmSession" {
				return nil
			}
			return tokenErr
		}),
		WithErrorHandler(func(_ context.Context, err error) {
			assert.Contains(t, err.Error(), "token invalid")
		}),
		WithConnectHook(func(closer io.Closer) {
			time.Sleep(time.Second)
			close(initSrv)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	client, err := Join(addr, WithSessionOpenHook(func(s Session) {
		initCli <- s
	}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, client.Connect())
	}()
	sess := <-initCli

	stream, err := sess.OpenStream("someStream")
	assert.Nil(t, stream)
	assert.Equal(t, err, tokenErr, err)
}

func TestWire_AuthSuccess(t *testing.T) {
	addr := genAddr(t)
	initSrv := make(chan struct{})
	initCli := make(chan struct{})

	wid := Identification("user")
	wtoken := Token("user")
	tokenErr := errors.New("token invalid")

	// server side
	server, err := Mount(addr,
		WithTokenValidator(func(streamName string, id Identification, token Token) error {
			assert.True(t, bytes.Equal(wid, id) && bytes.Equal(wtoken, token))
			// success
			if bytes.Equal(wid, id) && bytes.Equal(wtoken, token) {
				return nil
			}
			return tokenErr
		}),
		WithConnectHook(func(closer io.Closer) {
			time.Sleep(time.Second)
			close(initSrv)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	client, err := Join(addr,
		WithIdentification(Identification("user"), Token("user")),
		WithSessionOpenHook(func(s Session) {
			close(initCli)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, client.Connect())
	}()
	<-initCli
}

func TestWire_CloseAbort(t *testing.T) {
	addr := genAddr(t)
	initSrv := make(chan struct{})
	initCli := make(chan struct{})
	// server side
	server, err := Mount(addr,
		WithConnectHook(func(closer io.Closer) {
			assert.Nil(t, closer.Close())
		}),
		WithConnectHook(func(closer io.Closer) {
			time.Sleep(time.Second)
			close(initSrv)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	client, err := Join(addr,
		WithConnectHook(func(closer io.Closer) {
			assert.Nil(t, closer.Close())
		}),
		WithSessionOpenHook(func(s Session) {
			close(initCli)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, client.Connect())
	}()
	<-initCli

	assert.Equal(t, ErrWireClosed, client.Close())
	assert.Equal(t, ErrWireClosed, client.Close())
	assert.Nil(t, server.Close())
	assert.Nil(t, server.Close())
}

func TestWire_CloseSuccess(t *testing.T) {
	addr := genAddr(t)
	initSrv := make(chan struct{})
	initCli := make(chan struct{})
	// server side
	server, err := Mount(addr,
		WithConnectHook(func(closer io.Closer) {
			time.Sleep(time.Second)
			close(initSrv)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	client, err := Join(addr,
		WithSessionOpenHook(func(s Session) {
			close(initCli)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, client.Connect())
	}()
	<-initCli

	assert.Nil(t, client.Close())
	assert.Equal(t, ErrWireClosed, client.Close())

	assert.Nil(t, server.Close())
	assert.Nil(t, server.Close())

	assert.Len(t, client.Sessions(), 0)
	assert.Len(t, server.Sessions(), 0)
}

func TestWire_Shutdown(t *testing.T) {
	addr := genAddr(t)
	initSrv := make(chan struct{})
	initCli := make(chan Session)

	// server side
	server, err := Mount(addr,
		WithSessionCloseTimeout(5*time.Second),
		WithConnectHook(func(closer io.Closer) {
			time.Sleep(time.Second)
			close(initSrv)
		}))
	assert.Nil(t, err)
	server.Stream("ns:stream", func(ctx context.Context, s Stream) {
		time.Sleep(3 * time.Second)
		n, err := s.ReadFrom(bytes.NewReader([]byte("ok")))
		assert.Nil(t, err)
		assert.Equal(t, int64(2), n)
	})
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	client, err := Join(addr,
		WithSessionOpenHook(func(s Session) {
			initCli <- s
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, client.Connect())
	}()
	sess := <-initCli

	stream, err := sess.OpenStream("ns:stream")
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Close())
	}()
	buf := bytes.NewBuffer(nil)
	n, err := stream.WriteTo(buf)
	assert.Nil(t, err)
	assert.Equal(t, int64(2), n)
}

func TestWire_ShutdownFail(t *testing.T) {
	addr := genAddr(t)
	initSrv := make(chan struct{})
	initCli := make(chan Session)

	// server side
	server, err := Mount(addr,
		WithSessionCloseTimeout(time.Second),
		WithConnectHook(func(closer io.Closer) {
			time.Sleep(time.Second)
			close(initSrv)
		}))
	assert.Nil(t, err)
	server.Stream("ns:stream", func(ctx context.Context, s Stream) {
		time.Sleep(3 * time.Second)
		n, err := s.ReadFrom(bytes.NewReader([]byte("ok")))
		assert.Nil(t, err)
		assert.Equal(t, int64(2), n)
	})
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	client, err := Join(addr,
		WithSessionOpenHook(func(s Session) {
			initCli <- s
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, client.Connect())
	}()
	sess := <-initCli

	stream, err := sess.OpenStream("ns:stream")
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Close())
	}()
	buf := bytes.NewBuffer(nil)
	n, err := stream.WriteTo(buf)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), n)
	assert.Len(t, server.Sessions(), 0)
}

func TestWire_KeepAlive(t *testing.T) {
	addr := genAddr(t)
	initSrv := make(chan struct{})
	initCli := make(chan Session)

	// server side
	server, err := Mount(addr,
		WithConnectHook(func(closer io.Closer) {
			time.Sleep(time.Second)
			close(initSrv)
		}))
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, server.Connect())
	}()
	<-initSrv

	// client side
	client, err := Join(addr,
		WithKeepAlive(true),
		WithKeepAliveInterval(100*time.Millisecond),
		WithSessionOpenHook(func(s Session) {
			close(initCli)
		}),
		WithSessionCloseHook(func(s Session) {

		}),
	)
	assert.Nil(t, err)
	go func() {
		assert.Nil(t, client.Connect())
	}()
	<-initCli
	time.Sleep(time.Second)
}

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
