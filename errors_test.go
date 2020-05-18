package wirenet

import (
	"errors"
	"testing"

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
