package wirenet

import (
	"io"
	"time"
)

const (
	DefaultKeepAliveInterval   = 30 * time.Second
	DefaultEnableKeepAlive     = true
	DefaultReadTimeout         = 30 * time.Second
	DefaultWriteTimeout        = 30 * time.Second
	DefaultAcceptBacklog       = 256
	DefaultSessionCloseTimeout = 60 * time.Second
)

type Option func(*wire)

func WithLogWriter(w io.Writer) Option {
	return func(wire *wire) {
		wire.transportConf.LogOutput = w
	}
}

func WithKeepAliveInterval(interval time.Duration) Option {
	return func(wire *wire) {
		wire.transportConf.KeepAliveInterval = interval
		wire.transportConf.EnableKeepAlive = true
	}
}

func WithKeepAlive(flag bool) Option {
	return func(wire *wire) {
		wire.transportConf.EnableKeepAlive = flag
	}
}

func WithReadWriteTimeouts(read, write time.Duration) Option {
	return func(wire *wire) {
		wire.readTimeout = read
		wire.writeTimeout = write
		wire.transportConf.ConnectionWriteTimeout = write
	}
}

func WithCloseSessionTimeout(dur time.Duration) Option {
	return func(w *wire) {
		w.sessCloseTimeout = dur
	}
}
