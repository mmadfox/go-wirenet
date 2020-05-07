package main

import (
	"bytes"
	"log"
	"os/exec"

	"github.com/mediabuyerbot/go-wirenet"
)

type Ifconfig struct {
}

func (h Ifconfig) Name() string {
	return "ifconfig"
}

func (h Ifconfig) Serve(cmd wirenet.Cmd) error {
	cmdLocal := exec.Command("ifconfig")
	buf := bytes.NewBuffer(nil)
	cmdLocal.Stdout = buf
	if err := cmdLocal.Start(); err != nil {
		return err
	}
	if err := cmdLocal.Wait(); err != nil {
		return err
	}
	if _, err := cmd.ReadFrom(buf); err != nil {
		return err
	}
	return nil
}

func IfconfigCommand() Ifconfig {
	return Ifconfig{}
}

func main() {
	wire, err := wirenet.New(":9696", wirenet.ClientSide)
	if err != nil {
		log.Fatal(err)
	}
	defer wire.Close()

	wire.Mount(IfconfigCommand())

	log.Fatal(wire.Listen())
}
