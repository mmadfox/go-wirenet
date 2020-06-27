package wirenet_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/mediabuyerbot/go-wirenet"
)

func ExampleWireCallFromClient() {
	addr := fmt.Sprintf("127.0.0.1:%d", randomPort())
	serverIsOk := make(chan struct{})
	go func() {
		wire, err := wirenet.Mount(addr, wirenet.WithConnectHook(func(closer io.Closer) {
			close(serverIsOk)
		}))
		if err != nil {
			panic(err)
		}
		wire.Stream("ns:download", func(ctx context.Context, stream wirenet.Stream) {
			defer stream.Close()
			// write to stream
			data := make([]byte, 1024*1024)
			_, err := stream.ReadFrom(bytes.NewReader(data))
			if err != nil {
				panic(err)
			}
		})
		if err := wire.Connect(); err != nil {
			panic(err)
		}
	}()
	<-serverIsOk

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
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

			// call from client
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
		}()
	}
	wg.Wait()
	fmt.Print("OK")
	// Output: OK
}

func ExampleWireCallFromServer() {
	addr := fmt.Sprintf("127.0.0.1:%d", randomPort())
	serverIsOk := make(chan struct{})
	totalStreams := 10

	go func() {
		wire, err := wirenet.Mount(addr,
			wirenet.WithConnectHook(func(closer io.Closer) {
				close(serverIsOk)
			}))
		if err != nil {
			panic(err)
		}
		go func() {
			if err := wire.Connect(); err != nil {
				panic(err)
			}
		}()
		step := 0
		for step < 4 {
			time.Sleep(time.Second)
			step++
			if len(wire.Sessions()) != totalStreams {
				continue
			}
			break
		}
		buf := bytes.NewBuffer(nil)
		for _, sess := range wire.Sessions() {
			for _, name := range sess.StreamNames() {
				stream, err := sess.OpenStream(name)
				if err != nil {
					panic(err)
				}
				stream.WriteTo(buf)
			}
		}
		if buf.Len() == totalStreams*100 {
			log.Println("Download success!")
		}
	}()
	<-serverIsOk

	var wg sync.WaitGroup
	for i := 0; i < totalStreams; i++ {
		wg.Add(1)
		go func(cid int) {
			wire, err := wirenet.Join(addr)
			if err != nil {
				panic(err)
			}
			streamName := fmt.Sprintf("host%d:download", cid)
			wire.Stream(streamName, func(ctx context.Context, stream wirenet.Stream) {
				defer wg.Done()

				data := make([]byte, 100)
				n, err := stream.ReadFrom(bytes.NewReader(data))
				if err != nil {
					log.Printf("[ERROR] %v", err)
					return
				}
				if int(n) == len(data) {
					log.Printf("[INFO] download ok stream %s, session %s",
						streamName, stream.Session().ID())
				}
			})
			if err := wire.Connect(); err != nil {
				panic(err)
			}
		}(i)
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
