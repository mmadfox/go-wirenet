package wirenet

import (
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
)

func TestNewShutdownError(t *testing.T) {
	assert.NotNil(t, NewShutdownError())
}

func TestShutdownError_Error(t *testing.T) {
	err := NewShutdownError()
	err.Errors = append(err.Errors, errors.New("some error0"))
	err.Errors = append(err.Errors, errors.New("some error1"))
	err.Errors = append(err.Errors, errors.New("some error2"))
	assert.Equal(t, "session error some error0\nsession error some error1\nsession error some error2\n", err.Error())

	err = NewShutdownError()
	want := errors.New("error")
	err.Errors = append(err.Errors, want)
	assert.Equal(t, want.Error(), err.Error())
}

func TestShutdownError_HasErrors(t *testing.T) {
	err := NewShutdownError()
	err.Errors = append(err.Errors, errors.New("some error0"))
	err.Errors = append(err.Errors, errors.New("some error1"))
	err.Errors = append(err.Errors, errors.New("some error2"))
	assert.True(t, err.IsFilled())
}

func TestShutdownError_Add(t *testing.T) {
	someErr := errors.New("some error")
	err := NewShutdownError()
	err.Add(someErr)
	err.Add(someErr)
	err.Add(someErr)
	assert.Len(t, err.Errors, 3)
}

func TestOpError_Error(t *testing.T) {
	uid, _ := uuid.Parse("884a62fa-2e59-47c4-9238-ec46ec43be94")
	err := &OpError{
		Op:             "read stream",
		SessionID:      uid,
		Identification: Identification("id"),
		LocalAddr: &net.IPAddr{
			IP:   net.IP("0"),
			Zone: "",
		},
		RemoteAddr: &net.IPAddr{
			IP:   net.IP("0"),
			Zone: "",
		},
		Err: errors.New("some error"),
	}
	assert.Equal(t, "read stream sid-884a62fa-2e59-47c4-9238-ec46ec43be94 id-id ?30->?30: some error", err.Error())

	emptyErr := &OpError{}
	assert.Equal(t, "<nil>", emptyErr.Error())
	emptyErr = nil
	assert.Equal(t, "<nil>", emptyErr.Error())

	err = &OpError{
		Op:             "read stream",
		SessionID:      uid,
		Identification: Identification("id"),
		LocalAddr: &net.IPAddr{
			IP:   net.IP("0"),
			Zone: "",
		},
		RemoteAddr: nil,
		Err:        errors.New("some error"),
	}
	assert.Equal(t, "read stream sid-884a62fa-2e59-47c4-9238-ec46ec43be94 id-id ?30: some error", err.Error())
}
