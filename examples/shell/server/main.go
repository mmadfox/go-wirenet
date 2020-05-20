package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	addr := ":9099"
	wire, err := wirenet.Server(addr,
		wirenet.WithOpenSessionHook(func(session wirenet.Session) {
			log.Printf("open %s", session)
			exec(session)
		}),
		wirenet.WithCloseSessionHook(func(session wirenet.Session) {
			log.Printf("close %s", session)
		}),
	)
	if err != nil {
		panic(err)
	}

	wire.Mount("amount", func(_ context.Context, stream wirenet.Stream) {
		json.NewEncoder(stream).Encode("amount")
	})
	go func() {
		log.Println("shell server is running...")
		if err := wire.Connect(); err != nil {
			panic(err)
		}
	}()

	<-terminate()

	if err := wire.Close(); err != nil {
		log.Fatal(err)
	}
	log.Println("done")
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

func exec(session wirenet.Session) {
	for _, streamName := range session.StreamNames() {
		fmt.Printf("CALL REMOTE COMMAND %s FROM %s\n",
			streamName, session.ID())
		switch streamName {
		case "hostA:envPath", "hostB:envPath":
			envPath(streamName, session)
		case "ping":
			ping(session)
		case "ifconfig":
			ifconfig(session)
		}
		fmt.Printf("\n")
	}
	if err := session.Close(); err != nil {
		log.Println("close session error", err)
	}
}

func ping(session wirenet.Session) {
	stream, err := session.OpenStream("ping")
	if err != nil {
		log.Println("open stream error", err)
		return
	}
	defer stream.Close()
	stream.WriteTo(os.Stdout)
}

func ifconfig(session wirenet.Session) {
	stream, err := session.OpenStream("ifconfig")
	if err != nil {
		log.Println("open stream error", err)
		return
	}
	defer stream.Close()
	stream.WriteTo(os.Stdout)
}

func envPath(streamName string, session wirenet.Session) {
	stream, err := session.OpenStream(streamName)
	if err != nil {
		log.Println("open stream error", err)
		return
	}
	defer stream.Close()
	stream.WriteTo(os.Stdout)
}
