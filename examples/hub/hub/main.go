package main

import (
	"fmt"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	hub, err := wirenet.Hub(":8765",
		wirenet.WithSessionOpenHook(func(session wirenet.Session) {
			fmt.Printf("[HUB] session open %s\n", session.ID())
		}),
		wirenet.WithSessionCloseHook(func(session wirenet.Session) {
			fmt.Printf("[HUB] session close %s\n", session.ID())
		}),
	)
	if err != nil {
		panic(err)
	}

	if err := hub.Connect(); err != nil {
		panic(err)
	}
}
