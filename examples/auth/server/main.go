package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	validToken := wirenet.Token("token")
	wire, err := wirenet.Mount(":8989",
		wirenet.WithTokenValidator(func(streamName string, id wirenet.Identification, token wirenet.Token) error {
			fmt.Printf("[INFO] token validation: node=%s, stream=%s, token=%s\n",
				id, streamName, string(token))
			if bytes.Equal(validToken, token) {
				return nil
			}
			return errors.New("invalid token")
		}))
	if err != nil {
		panic(err)
	}

	go func() {
		fmt.Println("running")
		if err := wire.Connect(); err != nil {
			fmt.Printf("[ERROR] wire connect error %v\n", err)
		}
	}()

	<-terminate()

	fmt.Println("shutdown")
	wire.Close()
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
