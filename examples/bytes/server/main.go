package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "net/http/pprof"

	"github.com/mediabuyerbot/go-wirenet"
)

const bufSize = 1 << 20

func main() {
	addr := ":8765"
	wire, err := wirenet.Mount(addr)
	if err != nil {
		handleError(err)
	}

	payload := make([]byte, bufSize)
	rand.Read(payload)
	wire.Stream("bytes", func(ctx context.Context, stream wirenet.Stream) {
		_, err := stream.ReadFrom(bytes.NewReader(payload))
		if err != nil {
			fmt.Printf("[ERROR] readFrom error %v\n", err)
		}
	})

	go func() {
		http.ListenAndServe(":8080", nil)
	}()

	go func() {
		if err := wire.Connect(); err != nil {
			handleError(err)
		}
	}()

	<-terminate()

	if err := wire.Close(); err != nil {
		handleError(err)
	}
}

func handleError(err error) {
	fmt.Printf("[ERROR] %v", err)
	os.Exit(1)
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
