package cache

import (
	"go.uber.org/zap"
	"time"
)

const (
	defaultCleanerInterval = time.Second * 10
)

var nopLogger = zap.NewNop()

type Key interface {
	any
}

type Value interface {
	any
}

type Cache[K Key, V Value] interface {
	Close() error
	Get(key K) (value V, expirationTime time.Time, ok bool)
	Store(key K, value V, cacheTtl time.Duration)
	Range(f func(k K, v V, expirationTime time.Time) error) error
	Len() int
	Flush()
}
