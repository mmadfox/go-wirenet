package wirenet

import "time"

const (
	DefaultKeepAliveInterval = 30 * time.Second
	DefaultEnableKeepAlive   = true
	DefaultReadTimeout       = 30 * time.Second
	DefaultWriteTimeout      = 30 * time.Second
	DefaultAcceptBacklog     = 256
)

type Option func(*Wire)

func WithKeepAliveInterval(interval time.Duration) Option {
	return func(wire *Wire) {
		wire.transportConf.KeepAliveInterval = interval
		wire.transportConf.EnableKeepAlive = true
	}
}

func WithKeepAlive(flag bool) Option {
	return func(wire *Wire) {
		wire.transportConf.EnableKeepAlive = flag
	}
}

func WithReadWriteTimeouts(read, write time.Duration) Option {
	return func(wire *Wire) {
		wire.readTimeout = read
		wire.writeTimeout = write
		wire.transportConf.ConnectionWriteTimeout = write
	}
}
