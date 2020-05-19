package main

import (
	"io"
	"log"
	"os/exec"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	wire, err := wirenet.Client(":9099")
	if err != nil {
		panic(err)
	}
	wire.Mount("ping", func(stream wirenet.Stream) {
		if err := ping(stream); err != nil {
			log.Println("ping error", err)
		}
	})
	if err := wire.Connect(); err != nil {
		panic(err)
	}
}

func ping(w io.Writer) error {
	cmd := exec.Command("ping", "google.com")
	cmd.Stdout = w
	return cmd.Run()
}
