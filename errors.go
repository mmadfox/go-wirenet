package wirenet

import (
	"bytes"
	"errors"
)

var (
	// ErrStreamClosed is returned when named stream is closed.
	ErrStreamClosed = errors.New("wirenet: read/write on closed stream")

	// ErrSessionNotFound is returned when session not found.
	ErrSessionNotFound = errors.New("wirenet: session not found")

	// ErrWireClosed is returned when Wire is closed client or server side.
	ErrWireClosed = errors.New("wirenet: wire closed")

	// ErrAddrEmpty is returned when addr is empty. See Mount(), Join(), Hub().
	ErrAddrEmpty = errors.New("wirenet: listener address is empty")

	// ErrSessionClosed is returned when session is closed.
	ErrSessionClosed = errors.New("wirenet: session closed")

	// ErrUnknownCertificateName is returned when certificate name is empty. See LoadCertificates().
	ErrUnknownCertificateName = errors.New("wirenet: unknown certificate name")
)

// ShutdownError represent a container with errors.
// The container is used when closing the wire.
// If an error occurs during the sessions closing process, it is register to the container.
type ShutdownError struct {
	Errors []error
}

// NewShutdownError constructs a new ShutdownError.
func NewShutdownError() *ShutdownError {
	return &ShutdownError{
		Errors: make([]error, 0, 8),
	}
}

// Add adds an error to the container.
func (e *ShutdownError) Add(er error) {
	e.Errors = append(e.Errors, er)
}

// IsFilled returns a true flag if the container is full, otherwise returns a false flag.
func (e *ShutdownError) IsFilled() bool {
	return len(e.Errors) > 0
}

// Error returns a list of all errors in a string representation.
// Each error is separated by a symbol \n.
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
