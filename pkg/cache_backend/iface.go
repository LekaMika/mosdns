package cache_backend

import (
	"time"
)

const (
	DefaultLazyUpdateTimeout = time.Second * 5
	ExpiredMsgTtl            = 5
)

type CacheBackend[K Key, V interface{}] interface {
	Close() error
	Get(key K) (value V, expirationTime time.Time, ok bool)
	Store(key K, value V, cacheTtl time.Duration)
	Range(f func(key K, value V, expirationTime time.Time) error) error
	Len() int
	Flush()
	Delete(key K) error
}
