package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mediabuyerbot/go-wirenet"
)

type Quote struct {
	Symbol    string
	Price     int
	Timestamp int64
}

func main() {
	addr := ":9888"

	sesss := make(chan wirenet.Session, 1)
	wire, err := wirenet.Server(addr,
		wirenet.WithSessionCloseTimeout(5*time.Second),
		wirenet.WithOpenSessionHook(func(session wirenet.Session) {
			sesss <- session
		}))
	if err != nil {
		panic(err)
	}

	go aggregate(sesss)
	fmt.Println("Sample aggregate server")

	go func() {
		if err := wire.Connect(); err != nil {
			fmt.Printf("[ERROR] wire connect error %v\n", err)
		}
	}()

	<-terminate()

	fmt.Println("shutdown")
	wire.Close()
}

func aggregate(sessions chan wirenet.Session) {
	for sess := range sessions {
		for _, streamName := range sess.StreamNames() {
			switch streamName {
			case "binance:quotes", "google:quotes", "okcoin:quotes":
				go read(streamName, sess)
			}
		}
	}
}

func read(streamName string, session wirenet.Session) {
	stream, err := session.OpenStream(streamName)
	if err != nil {
		fmt.Printf("[ERROR] open stream %s  %v",
			streamName, err)
	}
	var quote Quote
	for {
		if err := json.NewDecoder(stream).Decode(&quote); err != nil {
			if err != io.EOF {
				fmt.Printf("[ERROR] json decode %v", err)
			}
			break
		}
		fmt.Printf("Quote:  exchange=%s, price=%d, symbol=%s, ts=%d\n",
			streamName, quote.Price, quote.Symbol, quote.Timestamp)
	}
	if err := session.Close(); err != nil {
		log.Println("close session error", err)
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
