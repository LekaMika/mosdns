package memory_cache

import (
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/cache"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"time"
)

func (c *MemoryCache) Get(key key) *cache.Item {
	value, _, _ := c.backend.Get(key)
	return value
}

func (c *MemoryCache) Store(key key, value *cache.Item, ttl time.Duration) {
	c.backend.Store(key, value, ttl)
}

func (c *MemoryCache) QueryDns(q *dns.Msg) (*dns.Msg, bool) {
	key := getMsgKey(q)
	return getRespFromCache(key, c.backend, false, 0)
}

func (c *MemoryCache) StoreDns(q *dns.Msg, r *dns.Msg) {
	key := getMsgKey(q)
	saveRespToCache(key, r, c.backend, 0)
}

func (c *MemoryCache) Close() error {
	if err := c.dumpCache(); err != nil {
		c.logger.Error("failed to dump cache", zap.Error(err))
	}
	c.closeOnce.Do(func() {
		close(c.closeNotify)
	})
	return c.backend.Close()
}

func (c *MemoryCache) Clean() error {
	return nil
}
