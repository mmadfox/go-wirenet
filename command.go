package wirenet

import (
	"errors"
	"io"
	"sync"

	"github.com/google/uuid"
)

type Cmd interface {
	io.Closer
}

type command struct {
	id       uuid.UUID
	sess     *session
	name     string
	isClosed bool
}

func newCommand(name string, sess *session) *command {
	cmd := &command{
		id:   uuid.New(),
		name: name,
		sess: sess,
	}

	sess.hub.register(cmd)
	return cmd
}

func (c *command) Close() error {
	if c.isClosed {
		return errors.New("command closed")
	}

	c.sess.hub.unregister(c)
	return nil
}

type commands struct {
	store   map[uuid.UUID]*command
	counter int
	sync.RWMutex
}

func newCommands() *commands {
	return &commands{
		store: make(map[uuid.UUID]*command),
	}
}

func (c *commands) total() int {
	c.RLock()
	defer c.RUnlock()
	return c.counter
}

func (c *commands) register(cmd *command) {
	c.Lock()
	defer c.Unlock()
	c.store[cmd.id] = cmd
	c.counter++
}

func (c *commands) unregister(cmd *command) {
	c.Lock()
	defer c.Unlock()
	delete(c.store, cmd.id)
	if c.counter > 0 {
		c.counter--
	}
}
