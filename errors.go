package wirenet

import (
	"bytes"
	"errors"
)

var (
	ErrStreamClosed           = errors.New("wirenet: read/write on closed stream")
	ErrSessionNotFound        = errors.New("wirenet: session not found")
	ErrStreamNotFound         = errors.New("wirenet: stream not found")
	ErrWireClosed             = errors.New("wirenet closed")
	ErrAddrEmpty              = errors.New("wirenet: listener address is empty")
	ErrUnknownRole            = errors.New("wirenet: unknown role")
	ErrSessionClosed          = errors.New("wirenet: session closed")
	ErrUnknownCertificateName = errors.New("wirenet: unknown certificate name")
)

type ShutdownError struct {
	Errors []error
}

func NewShutdownError() *ShutdownError {
	return &ShutdownError{
		Errors: make([]error, 0, 8),
	}
}

func (e *ShutdownError) Add(er error) {
	e.Errors = append(e.Errors, er)
}

func (e *ShutdownError) IsFilled() bool {
	return len(e.Errors) > 0
}

func (e *ShutdownError) Error() string {
	el := len(e.Errors)
	if el == 1 {
		return e.Errors[0].Error()
	}
	buf := bytes.NewBuffer(nil)
	for i := 0; i < el; i++ {
		buf.WriteString("session error " + e.Errors[i].Error() + "\n")
	}
	return buf.String()
}
