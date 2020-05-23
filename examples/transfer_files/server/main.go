package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	wire, err := wirenet.Point(":7989")
	if err != nil {
		panic(err)
	}

	wire.Stream("stats", func(ctx context.Context, stream wirenet.Stream) {
		for filename, fpath := range files() {
			transfer(filename, fpath, stream)
		}
	})

	if err := wire.Connect(); err != nil {
		panic(err)
	}
}

func transfer(filename, fp string, stream wirenet.Stream) {
	dir, _ := os.Getwd()
	fp = filepath.Join(dir, "examples/transfer_files/server/files", filename)
	file, err := os.Open(fp)
	if err != nil {
		fmt.Printf("[ERROR] %v\n", err)
		return
	}
	defer file.Close()

	// write filename
	writer := stream.Writer()
	writer.Write([]byte(filename))
	writer.Close()

	log.Println("OOO")

	// write body
	stream.ReadFrom(file)

	// close file
	file.Close()
}

func files() map[string]string {
	return map[string]string{
		"one": "files/one",
		"two": "files/two",
	}
}
