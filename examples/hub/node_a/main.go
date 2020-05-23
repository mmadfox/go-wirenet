package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	wire, err := wirenet.Join(":8765",
		wirenet.WithSessionOpenHook(func(session wirenet.Session) {
			fmt.Printf("[NODEA] session open %s\n", session.ID())
			stream, err := session.OpenStream("nodeB:hello")
			if err != nil {
				return
			}
			defer stream.Close()

			writer := stream.Writer()
			for i := 0; i < 10; i++ {
				writer.Write([]byte(fmt.Sprintf("%d. part1 Hello from nodeA\n", i)))
				time.Sleep(3 * time.Second)
			}
			writer.Close()

			for i := 10; i < 20; i++ {
				writer.Write([]byte(fmt.Sprintf("%d. part2 Hello from nodeA\n", i)))
				time.Sleep(3 * time.Second)
			}
			writer.Close()

			for i := 20; i < 30; i++ {
				writer.Write([]byte(fmt.Sprintf("%d. part3 Hello from nodeA\n", i)))
				time.Sleep(3 * time.Second)
			}
			writer.Close()

			log.Println("FIN")

		}),
		wirenet.WithSessionCloseHook(func(session wirenet.Session) {
			fmt.Printf("[NODEB] session close %s\n", session.ID())
		}),
	)
	if err != nil {
		panic(err)
	}

	wire.Stream("nodeA:hello", func(ctx context.Context, stream wirenet.Stream) {
		for !stream.IsClosed() {
			stream.WriteTo(os.Stdout)
		}
		log.Printf("FIN")
	})

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}
