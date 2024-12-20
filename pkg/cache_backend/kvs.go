package cache_backend

import (
	"github.com/IrineSistiana/mosdns/v5/pkg/concurrent_map"
	"hash/maphash"
)

type Key interface {
	any
	concurrent_map.Hashable
}

type Value interface {
	any
}

// -----------------------------------------------------------------------

type StringKey string

var seed = maphash.MakeSeed()

func (k StringKey) Sum() uint64 {
	return maphash.String(seed, string(k))
}

// -----------------------------------------------------------------------

type StringValue string

func (k StringValue) CacheBackendValueSum() uint64 {
	return maphash.String(seed, string(k))
}

type BytesValue []byte

func (k BytesValue) CacheBackendValueSum() uint64 {
	return maphash.Bytes(seed, k)
}

type BoolValue bool

func (k BoolValue) CacheBackendValueSum() uint64 {
	if k {
		return 1
	}
	return 0
}

//type Item struct {
//	Resp           *dns.Msg
//	StoredTime     time.Time
//	ExpirationTime time.Time
//}
//
//func (k Item) CacheBackendValueSum() uint64 {
//	return uint64(k.Resp.Id)
//}
