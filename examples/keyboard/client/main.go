package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	var (
		identification = flag.String("id", "client", "Identification")
		token          = flag.String("token", "token", "Token")
	)

	flag.Parse()

	id := wirenet.Identification(*identification)
	tk := wirenet.Token(*token)

	wire, err := wirenet.Join(":8976",
		wirenet.WithIdentification(id, tk))
	if err != nil {
		panic(err)
	}

	wire.Stream("shell", func(ctx context.Context, stream wirenet.Stream) {
		fmt.Println("[SHELL]")
		if err := shell(stream); err != nil {
			fmt.Printf("[ERROR] shell error %v\n", err)
		}
	})

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}

func shell(s wirenet.Stream) error {
	_, err := s.ReadFrom(bytes.NewReader(payload(1000)))
	return err
}

func payload(n int) []byte {
	b := make([]byte, n)
	var i int
	for i < n {
		b[i] = 'x'
		i++
	}
	return b
}
