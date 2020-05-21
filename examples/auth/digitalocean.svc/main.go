package main

import (
	"context"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	token := wirenet.Token("badtoken")
	identification := wirenet.Identification("digitalocean.svc")
	wire, err := wirenet.Client(":8989",
		wirenet.WithIdentification(identification, token),
	)
	if err != nil {
		panic(err)
	}

	wire.Mount("digitalocean:read", func(ctx context.Context, stream wirenet.Stream) {
		stream.Write([]byte("digitalocean some payload"))
	})

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}
