package main

import (
	"log"
	"os"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	wire, err := wirenet.Server(":9099",
		wirenet.WithOpenSessionHook(func(session wirenet.Session) {
			exec(session)
		}))
	if err != nil {
		panic(err)
	}
	if err := wire.Connect(); err != nil {
		panic(err)
	}
}

func exec(session wirenet.Session) {
	for _, streamName := range session.StreamNames() {
		switch streamName {
		case "ping":
			ping(session)
		case "ifconfig":
			ifconfig(session)
		}
	}
}

func ping(session wirenet.Session) {
	stream, err := session.OpenStream("ping")
	if err != nil {
		log.Println("open stream error", err)
		return
	}
	stream.WriteTo(os.Stdout)
}

func ifconfig(session wirenet.Session) {
	stream, err := session.OpenStream("ifconfig")
	if err != nil {
		log.Println("open stream error", err)
		return
	}
	stream.WriteTo(os.Stdout)
}
