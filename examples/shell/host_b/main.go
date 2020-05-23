package main

import (
	"context"
	"log"
	"os"
	"os/exec"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	wire, err := wirenet.Join(":9099")
	if err != nil {
		panic(err)
	}

	wire.Stream("ifconfig", func(_ context.Context, stream wirenet.Stream) {
		if err := ifconfig(stream); err != nil {
			log.Println("ifconfig error", err)
		}
	})

	wire.Stream("hostB:envPath", func(_ context.Context, stream wirenet.Stream) {
		if err := envPath(stream); err != nil {
			log.Println("envPath error", err)
		}
	})

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}

func ifconfig(s wirenet.Stream) error {
	cmd := exec.Command("ifconfig")
	writer := s.Writer()
	defer writer.Close()
	cmd.Stdout = writer
	return cmd.Run()
}

func envPath(s wirenet.Stream) error {
	cmd := exec.Command("echo", os.Getenv("PATH"))
	writer := s.Writer()
	defer writer.Close()
	cmd.Stdout = writer
	return cmd.Run()
}
