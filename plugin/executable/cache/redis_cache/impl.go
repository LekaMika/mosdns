package redis_cache

import (
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend"
	"github.com/miekg/dns"
	"strings"
	"time"
)

func (c *RedisCache) Get(key cache_backend.StringKey) string {
	value, _, _ := c.backend.Get(key)
	return value
}

func (c *RedisCache) Store(key cache_backend.StringKey, value string, ttl time.Duration) {
	c.backend.Store(key, value, ttl)
}

func (c *RedisCache) QueryDns(q *dns.Msg) (*dns.Msg, bool) {
	key := getMsgKey(q, c.args.Separator, c.args.Prefix)
	return c.getRespFromCache(key, false, 0)
}

func (c *RedisCache) StoreDns(q *dns.Msg, r *dns.Msg) {
	key := getMsgKey(q, c.args.Separator, c.args.Prefix)
	c.saveRespToCache(key, r, 0, "")
}

func (c *RedisCache) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeNotify)
	})
	return c.backend.Close()
}

func (c *RedisCache) Clean() error {
	if len(strings.TrimSpace(c.args.Prefix)) > 0 && len(strings.TrimSpace(c.args.Separator)) > 0 {
		return c.backend.Delete(cache_backend.StringKey(fmt.Sprintf("%s%s*", c.args.Prefix, c.args.Separator)))
	}
	return nil
}
