package wirenet

import (
	"crypto/tls"
	"io"
	"math"
	"time"
)

const (
	DefaultKeepAliveInterval   = 15 * time.Second
	DefaultEnableKeepAlive     = true
	DefaultReadTimeout         = 10 * time.Second
	DefaultWriteTimeout        = 10 * time.Second
	DefaultAcceptBacklog       = 256
	DefaultSessionCloseTimeout = 5 * time.Second
	DefaultRetryMax            = 100
	DefaultRetryWaitMax        = 60 * time.Second
	DefaultRetryWaitMin        = 5 * time.Second
)

type (
	Option         func(*wire)
	Identification []byte
	Token          []byte
)

func WithConnectHook(hook func(io.Closer)) Option {
	return func(w *wire) {
		w.onConnect = hook
	}
}

func WithSessionOpenHook(hook SessionHook) Option {
	return func(w *wire) {
		w.openSessHook = hook
	}
}

func WithSessionCloseHook(hook SessionHook) Option {
	return func(w *wire) {
		w.closeSessHook = hook
	}
}

func WithIdentification(id Identification, token Token) Option {
	return func(w *wire) {
		w.identification = id
		w.token = token
	}
}

func WithTokenValidator(v TokenValidator) Option {
	return func(w *wire) {
		w.verifyToken = v
	}
}

func WithTLS(conf *tls.Config) Option {
	return func(w *wire) {
		w.tlsConfig = conf
	}
}

func WithRetryWait(min, max time.Duration) Option {
	return func(w *wire) {
		w.retryWaitMax = max
		w.retryWaitMin = min
	}
}

func WithRetryMax(n int) Option {
	return func(w *wire) {
		w.retryMax = n
	}
}

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

func WithSessionCloseTimeout(dur time.Duration) Option {
	return func(w *wire) {
		w.sessCloseTimeout = dur
	}
}

func DefaultRetryPolicy(min, max time.Duration, attemptNum int) time.Duration {
	m := math.Pow(2, float64(attemptNum)) * float64(min)
	wait := time.Duration(m)
	if float64(wait) != m || wait > max {
		wait = max
	}
	return wait
}
