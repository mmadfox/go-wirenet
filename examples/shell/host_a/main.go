package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	wire, err := wirenet.Client(":9099")
	if err != nil {
		panic(err)
	}
	wire.Mount("ping", func(stream wirenet.Stream) {
		fmt.Printf("retrieve from server: %s \n", amountFromServer(wire))
		if err := ping(stream); err != nil {
			log.Println("ping error", err)
		}
	})
	wire.Mount("hostA:envPath", func(stream wirenet.Stream) {
		if err := envPath(stream); err != nil {
			log.Println("envPath error", err)
		}
	})
	if err := wire.Connect(); err != nil {
		panic(err)
	}
}

func amountFromServer(w wirenet.Wire) (str string) {
	amount, err := w.Stream("amount")
	if err != nil {
		log.Println("amount from server error", err)
		return ""
	}
	defer amount.Close()
	json.NewDecoder(amount).Decode(&str)
	return
}

func ping(w io.Writer) error {
	cmd := exec.Command("ping", "google.com", "-c", "10")
	cmd.Stdout = w
	return cmd.Run()
}

func envPath(w io.Writer) error {
	cmd := exec.Command("echo", os.Getenv("PATH"))
	cmd.Stdout = w
	return cmd.Run()
}
