package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	sessions := make(chan wirenet.Session, 1)
	wire, err := wirenet.Join(":7989",
		wirenet.WithSessionOpenHook(func(session wirenet.Session) {
			sessions <- session
		}))
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			sess := <-sessions
			log.Printf("%s download...", sess.ID())

			downloadFiles(sess)
		}
	}()

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}

func downloadFiles(sess wirenet.Session) {
	stream, err := sess.OpenStream("stats")
	if err != nil {
		log.Println("open stream", err)
		return
	}
	defer stream.Close()

	readFile(stream)
	readFile(stream)

	sess.CloseWire()
}

func readFile(stream wirenet.Stream) {
	reader := stream.Reader()
	fn := make([]byte, 32)
	for {
		_, err := reader.Read(fn)
		if err != nil {
			break
		}
	}
	reader.Close()

	_, err := stream.WriteTo(os.Stdout)
	if err == nil {
		fmt.Printf("[INFO] transfer to %s OK\n", string(fn))
	} else {
		fmt.Printf("[ERROR] transfer error %v\n", err)
	}
}
