package main

import (
	"crypto/tls"
	"os"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	tlsConf, err := wirenet.LoadCertificates("client", "./certs")
	if err != nil {
		panic(err)
	}

	session := make(chan wirenet.Session)
	wire, err := wirenet.Join(":9076", opts(session, tlsConf)...)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := wire.Connect(); err != nil {
			panic(err)
		}
	}()

	sess := <-session
	info, err := sess.OpenStream("info")
	if err != nil {
		panic(err)
	}

	info.WriteTo(os.Stdout)
}

func opts(connCh chan wirenet.Session, tlsConf *tls.Config) []wirenet.Option {
	tlsConf.InsecureSkipVerify = true
	return []wirenet.Option{
		wirenet.WithTLS(tlsConf),
		wirenet.WithIdentification(wirenet.Identification("client"), nil),
		wirenet.WithSessionCloseHook(func(session wirenet.Session) {
			connCh <- session
			close(connCh)
		}),
	}
}
