/*
 * Copyright (C) 2024, Vizaxe
 *
 * This file is part of mosdns.
 *
 * mosdns is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * mosdns is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package redis_cache

import (
	"context"
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend"
	"github.com/IrineSistiana/mosdns/v5/pkg/dnsutils"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/cache"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

func (c *RedisCache) doLazyUpdate(msgKey string, qCtx *query_context.Context, next sequence.ChainWalker) {
	qCtxCopy := qCtx.Copy()
	lazyUpdateFunc := func() (any, error) {
		defer c.lazyUpdateSF.Forget(msgKey)
		qCtx := qCtxCopy

		c.logger.Debug("start lazy cache update", qCtx.InfoField())
		ctx, cancel := context.WithTimeout(context.Background(), cache_backend.DefaultLazyUpdateTimeout)
		defer cancel()

		err := next.ExecNext(ctx, qCtx)
		if err != nil {
			c.logger.Warn("failed to update lazy cache", qCtx.InfoField(), zap.Error(err))
		}

		r := qCtx.R()
		if r != nil {
			c.saveRespToCache(msgKey, r, c.args.LazyCacheTTL, qCtx.GetBlackHoleTag())
			c.updatedKey.Add(1)
		}
		c.logger.Debug("lazy cache updated", qCtx.InfoField())
		return nil, nil
	}
	c.lazyUpdateSF.DoChan(msgKey, lazyUpdateFunc) // DoChan won't block this goroutine
}

// saveRespToCache saves r to cache backend. It returns false if r
// should not be cached and was skipped.
func (c *RedisCache) saveRespToCache(msgKey string, r *dns.Msg, lazyCacheTtl int, blackHoleTag string) bool {
	if r.Truncated != false {
		return false
	}

	var msgTtl time.Duration
	var cacheTtl time.Duration
	switch r.Rcode {
	case dns.RcodeNameError:
		msgTtl = time.Second * 30
		cacheTtl = msgTtl
	case dns.RcodeServerFailure:
		msgTtl = time.Second * 5
		cacheTtl = msgTtl
	case dns.RcodeSuccess:
		minTTL := dnsutils.GetMinimalTTL(r)
		if len(r.Answer) == 0 { // Empty answer. Set ttl between 0~300.
			const maxEmptyAnswerTtl = 300
			msgTtl = time.Duration(min(minTTL, maxEmptyAnswerTtl)) * time.Second
			if lazyCacheTtl == redis.KeepTTL {
				cacheTtl = redis.KeepTTL
			} else {
				cacheTtl = msgTtl
			}
		} else {
			msgTtl = time.Duration(minTTL) * time.Second
			if lazyCacheTtl == redis.KeepTTL {
				cacheTtl = redis.KeepTTL
			} else if lazyCacheTtl > 0 {
				cacheTtl = time.Duration(lazyCacheTtl) * time.Second
			} else {
				cacheTtl = msgTtl
			}
		}
	}
	if msgTtl <= 0 || (cacheTtl <= 0 && cacheTtl != redis.KeepTTL) {
		return false
	}

	now := time.Now()
	expirationTime := now.Add(msgTtl)
	v := &cache.Item{
		Resp:           setDefaultVal(r),
		StoredTime:     now,
		ExpirationTime: expirationTime,
		BlockHoleTag:   blackHoleTag,
	}
	msg := marshalItem(v)
	c.backend.Store(cache_backend.StringKey(msgKey), msg, cacheTtl)
	return true
}

// getRespFromCache returns the cached response from cache.
// The ttl of returned msg will be changed properly.
// Returned bool indicates whether this response is hit by lazy cache.
// Note: Caller SHOULD change the msg id because it's not same as query's.
func (c *RedisCache) getRespFromCache(msgKey string, lazyCacheEnabled bool, lazyTtl int) (*dns.Msg, bool) {
	// Lookup cache
	v, _, ok := c.backend.Get(cache_backend.StringKey(msgKey))
	item := unmarshalItem(v)
	// Cache hit
	if ok && item != nil {
		now := time.Now()

		expirationTime := item.ExpirationTime
		storedTime := item.StoredTime
		resp := setDefaultVal(item.Resp)
		// Not expired.
		if now.Before(expirationTime) {
			r := resp
			dnsutils.SubtractTTL(r, uint32(now.Sub(storedTime).Seconds()))
			return r, false
		}

		// Msg expired but cache isn't. This is a lazy cache enabled entry.
		// If lazy cache is enabled, return the response.
		if lazyCacheEnabled {
			r := resp
			dnsutils.SetTTL(r, uint32(lazyTtl))
			return r, true
		}
	}

	// cache miss
	return nil, false
}
