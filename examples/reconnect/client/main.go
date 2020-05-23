package main

import (
	"encoding/binary"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mediabuyerbot/go-wirenet"
)

var startIndex int

func main() {
	wire, err := wirenet.Client(":9876",
		wirenet.WithSessionCloseTimeout(time.Second),
		wirenet.WithRetryMax(1000),
		wirenet.WithRetryWait(time.Second, 10*time.Second),
		wirenet.WithOpenSessionHook(func(session wirenet.Session) {
			log.Println("open sess", session.ID())
			read(session)
		}),
		wirenet.WithCloseSessionHook(func(session wirenet.Session) {
			log.Println("close sess", session.ID())
		}))
	if err != nil {
		panic(err)
	}

	go func() {
		if err := wire.Connect(); err != nil {
			panic(err)
		}
		os.Exit(0)
	}()

	<-terminate()

	if err := wire.Close(); err != nil {
		log.Fatal(err)
	}
}

func read(sess wirenet.Session) {
	stream, err := sess.OpenStream("sayHello")
	if err != nil {
		return
	}
	defer stream.Close()

	writer := stream.Writer()
	if err := binary.Write(writer, binary.LittleEndian, uint32(startIndex)); err != nil {
		log.Println("write error", err)
	}
	writer.Close()

	reader := stream.Reader()
	buf := make([]byte, 32)
	for {
		n, err := reader.Read(buf)
		if err != nil || n == 0 {
			break
		}
		os.Stdout.Write(buf)
		startIndex++
	}
	reader.Close()

	if startIndex >= 20 {
		log.Println("data complete!")
		sess.CloseWire()
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
