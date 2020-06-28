package wirenet

import (
	"bytes"
	"errors"
	"net"

	"github.com/google/uuid"
)

var (
	// ErrStreamClosed is returned when named stream is closed.
	ErrStreamClosed = errors.New("wirenet: read/write on closed stream")

	// ErrStreamHandlerNotFound is returned when stream handler not found.
	ErrStreamHandlerNotFound = errors.New("wirenet: stream handler not found")

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

type OpError struct {
	Op             string
	SessionID      uuid.UUID
	LocalAddr      net.Addr
	RemoteAddr     net.Addr
	Identification Identification
	Err            error
}

func (e *OpError) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.Op
	s = s + " sid-" + e.SessionID.String()
	if len(e.Identification) > 0 {
		s = s + " id-" + string(e.Identification)
	}
	if e.LocalAddr != nil {
		s += " " + e.LocalAddr.String()
	}
	if e.RemoteAddr != nil {
		if e.RemoteAddr != nil {
			s += "->"
		} else {
			s += " "
		}
		s += e.RemoteAddr.String()
	}
	s += ": " + e.Err.Error()
	return s
}

// ShutdownError is the error type with errors that occurred when closing a sessions.
// The error is used only when closing a wired connection.
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
