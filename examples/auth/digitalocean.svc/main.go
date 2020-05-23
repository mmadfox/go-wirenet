package main

import (
	"context"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	token := wirenet.Token("badtoken")
	identification := wirenet.Identification("digitalocean.svc")
	wire, err := wirenet.Join(":8989",
		wirenet.WithIdentification(identification, token),
	)
	if err != nil {
		panic(err)
	}

	wire.Stream("digitalocean:read", func(ctx context.Context, stream wirenet.Stream) {
		writer := stream.Writer()
		writer.Write([]byte("digitalocean some payload"))
		writer.Close()
	})

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}
