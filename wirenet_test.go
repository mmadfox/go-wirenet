package wirenet

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWire_OpenSession(t *testing.T) {
	port, err := RandomPort()
	assert.Nil(t, err)

	var total int32
	iter := 50

	addr := fmt.Sprintf(":%d", port)
	wire, err := New(addr, ServerSide)
	assert.Nil(t, err)
	wire.OpenSession(func(s Session) error {
		atomic.AddInt32(&total, 1)
		return nil
	})
	go func() {
		assert.Nil(t, wire.Listen())
	}()

	time.Sleep(time.Second)

	var wg sync.WaitGroup
	for i := 0; i < iter; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wire, err := New(addr, ClientSide)
			assert.Nil(t, err)
			wire.OpenSession(func(s Session) error {
				_ = wire.Close()
				return nil
			})
			assert.Nil(t, err)
			assert.Nil(t, wire.Listen())
		}()
	}
	wg.Wait()
	assert.Nil(t, wire.Close())
	assert.Equal(t, iter, int(total))
	t.Logf("total sessions %d", total)
}
