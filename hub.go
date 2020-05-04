package wirenet

import (
	"sync"

	"github.com/google/uuid"
)

type commandHub interface {
	Len() int
	Register(cmd *command)
	Unregister(cmd *command)
	Close()
}

type sessionHub interface {
	Len() int
	Register(sess *session)
	Unregister(sess *session)
	Each(func(sess *session))
}

type cmdHub struct {
	store map[uuid.UUID]*command
	sync.RWMutex
}

func newCommandHub() commandHub {
	return &cmdHub{
		store: make(map[uuid.UUID]*command),
	}
}

func (c *cmdHub) Len() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.store)
}

func (c *cmdHub) Register(cmd *command) {
	c.Lock()
	defer c.Unlock()
	c.store[cmd.id] = cmd
}

func (c *cmdHub) Unregister(cmd *command) {
	c.Lock()
	defer c.Unlock()
	delete(c.store, cmd.id)
}

func (c *cmdHub) Close() {
	c.RLock()
	commands := c.store
	c.RUnlock()
	for _, cmd := range commands {
		_ = cmd.Close()
	}
}

type sessHub struct {
	store map[uuid.UUID]*session
	sync.RWMutex
}

func newSessionHub() sessionHub {
	return &sessHub{
		store: make(map[uuid.UUID]*session),
	}
}

func (s *sessHub) Len() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.store)
}

func (s *sessHub) Register(sess *session) {
	s.Lock()
	defer s.Unlock()
	s.store[sess.id] = sess
}

func (s *sessHub) Unregister(sess *session) {
	s.Lock()
	defer s.Unlock()
	delete(s.store, sess.id)
}

func (s *sessHub) Each(fn func(sess *session)) {
	s.RLock()
	sessions := s.store
	s.RUnlock()
	for _, sess := range sessions {
		fn(sess)
	}
}
