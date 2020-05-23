package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	tlsConf, err := wirenet.LoadCertificates("server", "./certs")
	if err != nil {
		panic(err)
	}

	wire, err := wirenet.Mount(":9076",
		wirenet.WithTLS(tlsConf),
		wirenet.WithSessionOpenHook(func(session wirenet.Session) {
			fmt.Printf("[INFO] session=%s, identification=%s\n",
				session.ID(), string(session.Identification()))
		}))
	if err != nil {
		panic(err)
	}

	wire.Stream("info", func(ctx context.Context, stream wirenet.Stream) {
		for !stream.IsClosed() {
			time.Sleep(time.Second)
			writer := stream.Writer()
			writer.Write([]byte("HELLO FROM SERVER!"))
			writer.Close()
		}
	})

	fmt.Println("running...")

	if err := wire.Connect(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
