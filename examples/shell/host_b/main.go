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
	wire.Mount("ifconfig", func(stream wirenet.Stream) {
		if err := ifconfig(stream); err != nil {
			log.Println("ifconfig error", err)
		}
	})
	if err := wire.Connect(); err != nil {
		panic(err)
	}
}

func ifconfig(w io.Writer) error {
	cmd := exec.Command("ifconfig")
	cmd.Stdout = w
	return cmd.Run()
}
