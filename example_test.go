package wirenet_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/mediabuyerbot/go-wirenet"
)

func wireServer(addr string, isOkCh chan struct{}) {
	wire, err := wirenet.Mount(addr, wirenet.WithConnectHook(func(closer io.Closer) {
		close(isOkCh)
	}))
	if err != nil {
		panic(err)
	}
	wire.Stream("ns:download", func(ctx context.Context, stream wirenet.Stream) {
		defer stream.Close()
		data := make([]byte, 1024*1024)
		_, err := stream.ReadFrom(bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
	})
	if err := wire.Connect(); err != nil {
		panic(err)
	}
}

func wireClient(addr string, wg *sync.WaitGroup) {
	defer wg.Done()

	sessCh := make(chan wirenet.Session)
	wire, err := wirenet.Join(addr, wirenet.WithSessionOpenHook(func(session wirenet.Session) {
		sessCh <- session
	}))
	if err != nil {
		panic(err)
	}
	go func() {
		if err := wire.Connect(); err != nil {
			panic(err)
		}
	}()
	sess := <-sessCh
	log.Println("open session", sess.ID())

	stream, err := sess.OpenStream("ns:download")
	if err != nil {
		panic(err)
	}
	defer stream.Close()
	buf := bytes.NewBuffer(nil)
	n, err := stream.WriteTo(buf)
	if err != nil {
		panic(err)
	}
	if int(n) == buf.Len() {
		log.Println("download OK", sess.ID())
	}
}

func ExampleWire() {
	addr := fmt.Sprintf("127.0.0.1:%d", randomPort())
	serverIsOk := make(chan struct{})
	go wireServer(addr, serverIsOk)
	<-serverIsOk

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go wireClient(addr, &wg)
	}
	wg.Wait()
	fmt.Print("OK")
	// Output: OK
}

func randomPort() int {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		panic(err)
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
