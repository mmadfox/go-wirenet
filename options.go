package wirenet

import (
	"crypto/tls"
	"io"
	"math"
	"time"
)

const (
	DefaultKeepAliveInterval   = 30 * time.Second
	DefaultEnableKeepAlive     = true
	DefaultReadTimeout         = 30 * time.Second
	DefaultWriteTimeout        = 30 * time.Second
	DefaultAcceptBacklog       = 256
	DefaultSessionCloseTimeout = 10 * time.Second
	DefaultRetryMax            = 100
	DefaultRetryWaitMax        = 60 * time.Second
	DefaultRetryWaitMin        = 1 * time.Second
)

type Option func(*wire)

func WithOnConnect(fn func(io.Closer)) Option {
	return func(w *wire) {
		w.onListen = fn
	}
}

func WithOpenSessionHook(hook SessionHook) Option {
	return func(w *wire) {
		w.openSessHook = hook
	}
}

func WithCloseSessionHook(hook SessionHook) Option {
	return func(w *wire) {
		w.closeSessHook = hook
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
		if n <= 0 {
			n = 1
		}
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

func WithCloseSessionTimeout(dur time.Duration) Option {
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
