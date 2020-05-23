package main

import (
	"bytes"
	"context"
	"log"
	"os"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	wire, err := wirenet.Join(":8765")
	if err != nil {
		panic(err)
	}

	wire.Stream("nodeB:hello", func(ctx context.Context, stream wirenet.Stream) {
		log.Println("NodeB stream init")

		str, _ := stream.Session().OpenStream("nodeA:hello")
		str.ReadFrom(bytes.NewReader([]byte("GGGGG")))

		for !stream.IsClosed() {
			stream.WriteTo(os.Stdout)
		}
		log.Printf("FIN")
	})

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}
