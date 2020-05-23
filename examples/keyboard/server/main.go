package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	wire, err := wirenet.Mount(":8976", nil)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			command := readCommand()
			if len(command) < 2 {
				continue
			}

			for sid, sess := range wire.Sessions() {
				fmt.Printf("[EXEC] command=%s, session=%s, identification=%s\n",
					command, sid, sess.Identification())

				if err := exec(command, sess); err != nil {
					fmt.Printf("[ERROR] %v\n", err)
				}
			}
		}
	}()

	go func() {
		if err := wire.Connect(); err != nil {
			panic(err)
		}
	}()

	<-terminate()

	wire.Close()
}

func exec(command string, sess wirenet.Session) error {
	stream, err := sess.OpenStream("shell")
	if err != nil {
		return err
	}
	defer stream.Close()

	payload := bytes.NewBuffer(nil)
	payload.WriteString(command)
	_, err = stream.ReadFrom(payload)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(nil)
	_, err = stream.WriteTo(buf)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func readCommand() string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("enter command:")
	text, _ := reader.ReadString('\n')
	return strings.Trim(text, " ")
}

func terminate() chan struct{} {
	done := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		done <- struct{}{}
	}()
	return done
}
