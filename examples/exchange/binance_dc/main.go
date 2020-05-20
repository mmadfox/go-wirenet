package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
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
	wire, err := wirenet.Client(addr)
	if err != nil {
		panic(err)
	}

	wait := func() {
		time.Sleep(time.Second)
	}

	wire.Mount("binance:quotes", func(ctx context.Context, stream wirenet.Stream) {
		for {
			wait()

			if err := json.NewEncoder(stream).Encode(Quote{
				Price:     rand.Intn(1000),
				Timestamp: time.Now().Unix(),
				Symbol:    "USD",
			}); err != nil {
				fmt.Printf("[ERROR] json encode error %v", err)
				return
			}
		}
	})

	wire.Connect()
}
