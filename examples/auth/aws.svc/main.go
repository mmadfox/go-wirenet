package main

import (
	"context"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	token := wirenet.Token("token")
	identification := wirenet.Identification("aws.svc")
	wire, err := wirenet.Join(":8989",
		wirenet.WithIdentification(identification, token),
	)
	if err != nil {
		panic(err)
	}

	wire.Stream("aws:read", func(ctx context.Context, stream wirenet.Stream) {
		writer := stream.Writer()
		writer.Write([]byte("aws some payload"))
		writer.Close()
	})

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}
