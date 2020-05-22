package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	wire, err := wirenet.Server(":9876", opts()...)
	if err != nil {
		panic(err)
	}

	wire.Mount("sayHello", func(ctx context.Context, stream wirenet.Stream) {
		var startIndex uint32
		if err := binary.Read(stream, binary.LittleEndian, &startIndex); err != nil {
			log.Println("read error", err)
			return
		}
		for i := startIndex; ; i++ {
			if i > 20 {
				break
			}
			payload := []byte(fmt.Sprintf("%d. Hello from server! ", i))
			n, err := stream.Write(payload)
			if err != nil || n == 0 {
				return
			}
			time.Sleep(time.Second)
		}
	})

	go func() {
		log.Println("running...")
		if err := wire.Connect(); err != nil {
			panic(err)
		}
	}()

	<-terminate()

	if err := wire.Close(); err != nil {
		log.Fatal(err)
	}
}

func terminate() chan struct{} {
	done := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		done <- struct{}{}
	}()
	return done
}

func opts() []wirenet.Option {
	return []wirenet.Option{
		wirenet.WithOpenSessionHook(func(session wirenet.Session) {
			log.Printf("open session id=%s", session.ID())
		}),
		wirenet.WithCloseSessionHook(func(session wirenet.Session) {
			log.Printf("close session id=%s", session.ID())
		}),
	}
}
