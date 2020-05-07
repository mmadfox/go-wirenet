package main

import (
	"log"

	"github.com/mediabuyerbot/go-wirenet"
)

func ifconfig(sess wirenet.Session) (s string, err error) {
	cmd, err := sess.Command("ifconfig")
	if err != nil {
		return s, sess.Close()
	}
	defer cmd.Close()
	b, err := cmd.Call(nil)
	if err != nil {
		return s, err
	}
	return string(b), nil
}

func main() {
	wire, err := wirenet.New(":9696", wirenet.ServerSide)
	if err != nil {
		log.Fatal(err)
	}
	defer wire.Close()

	wire.OpenSession(func(session wirenet.Session) error {
		log.Printf("open session id %v", session.ID())

		out, err := ifconfig(session)
		if err != nil {
			log.Printf("[ERROR] %v", err)
		}
		log.Println(out)
		return nil
	})

	wire.CloseSession(func(session wirenet.Session) error {
		log.Printf("close session id %v", session.ID())
		return nil
	})

	log.Fatal(wire.Listen())
}
