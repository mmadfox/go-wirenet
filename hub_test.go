package wirenet

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
)

func TestCmdHub(t *testing.T) {
	hub := newCommandHub()
	stream := NopCloser(bytes.NewBuffer(nil))
	max := 5
	for i := 0; i < max; i++ {
		newCommand(fmt.Sprintf("cmd-%d", i), stream, hub)
	}
	assert.Equal(t, max, hub.Len())
	hub.Close()
	assert.Equal(t, 0, hub.Len())
}

func TestSessHub(t *testing.T) {
	hub := newSessionHub()
	max := 5
	for i := 0; i < max; i++ {
		hub.Register(&session{id: uuid.New()})
	}
	assert.Equal(t, max, hub.Len())
	for _, sess := range hub.List() {
		hub.Unregister(sess)
	}
	assert.Equal(t, 0, hub.Len())
}
