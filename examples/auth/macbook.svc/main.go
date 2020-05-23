package main

import (
	"context"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	token := wirenet.Token("token")
	identification := wirenet.Identification("macbook.svc")
	wire, err := wirenet.Client(":8989",
		wirenet.WithIdentification(identification, token),
	)
	if err != nil {
		panic(err)
	}

	wire.Mount("macbook:read", func(ctx context.Context, stream wirenet.Stream) {
		writer := stream.Writer()
		writer.Write([]byte("macbook some payload"))
		writer.Close()
	})

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}
