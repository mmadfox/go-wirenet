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
	wire, err := wirenet.Join(addr)
	if err != nil {
		panic(err)
	}

	wait := func() {
		time.Sleep(time.Second)
	}

	wire.Stream("google:quotes", func(ctx context.Context, stream wirenet.Stream) {
		writer := stream.Writer()
		for {
			wait()

			if err := json.NewEncoder(writer).Encode(Quote{
				Price:     rand.Intn(10000),
				Timestamp: time.Now().Unix(),
				Symbol:    "BTC",
			}); err != nil {
				fmt.Printf("[ERROR] json encode error %v", err)
				return
			}
			writer.Close()
		}
	})

	wire.Connect()
}
