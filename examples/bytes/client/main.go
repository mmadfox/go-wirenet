package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mediabuyerbot/go-wirenet"

	_ "net/http/pprof"
)

func main() {
	addr := ":8765"
	sess := make(chan wirenet.Session)

	for i := 0; i < 100; i++ {
		go func() {
			wire, err := wirenet.Join(addr, wirenet.WithSessionOpenHook(func(session wirenet.Session) {
				sess <- session
			}))
			if err != nil {
				handleError(err)
			}
			if err := wire.Connect(); err != nil {
				handleError(err)
			}
		}()
	}

	go func() {
		http.ListenAndServe(":8081", nil)
	}()

	go func() {
		for {
			s := <-sess
			fmt.Printf("open session %s\n", s.ID())
			go func(s wirenet.Session) {
				buf := bytes.NewBuffer(nil)
				for {
					buf.Reset()

					stream, err := s.OpenStream("bytes")
					if err != nil {
						handleError(err)
					}

					if _, err := stream.WriteTo(buf); err != nil {
						handleError(err)
					}
					if err := stream.Close(); err != nil {
						handleError(err)
					}
					fmt.Printf("[INFO] %s read %d bytes\n", s.ID(), buf.Len())
					time.Sleep(500 * time.Millisecond)
				}
			}(s)
		}
	}()

	<-terminate()
}

func handleError(err error) {
	fmt.Printf("[ERROR] %v", err)
	os.Exit(1)
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
