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
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend"
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend/redis_cache_backend"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"sync"
	"sync/atomic"
)

const (
	PluginType = "redis_cache"
)

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
}

var _ sequence.RecursiveExecutable = (*RedisCache)(nil)

type Args struct {
	Url          string `yaml:"url"`
	RedisTimeout int    `yaml:"redis_timeout"`
	LazyCacheTTL int    `yaml:"lazy_cache_ttl"`
	Separator    string `yaml:"separator"`
	Prefix       string `yaml:"prefix"`
	StoreOnly    bool   `yaml:"store_only"`
}

func (a *Args) init() {
	if &a.Separator == nil || len(a.Separator) == 0 {
		a.Separator = ":"
	}
}

type RedisCache struct {
	args *Args

	logger       *zap.Logger
	backend      cache_backend.CacheBackend[cache_backend.StringKey, string]
	lazyUpdateSF singleflight.Group
	closeOnce    sync.Once
	closeNotify  chan struct{}
	updatedKey   atomic.Uint64

	queryTotal   prometheus.Counter
	hitTotal     prometheus.Counter
	lazyHitTotal prometheus.Counter
	size         prometheus.GaugeFunc
}

func Init(bp *coremain.BP, args any) (any, error) {
	c, err := NewRedisCache(args.(*Args), bp.Tag(), bp.L())
	if err != nil {
		return nil, err
	}

	return c, nil
}

func NewRedisCache(args *Args, tag string, logger *zap.Logger) (*RedisCache, error) {

	args.init()

	if logger == nil {
		logger = zap.NewNop()
	}
	// serial initialization
	backend, err := redis_cache_backend.NewRedisCache(args.Url)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis cache, %w", err)
	}
	lb := map[string]string{"tag": tag}
	p := &RedisCache{
		args: args,

		logger:      logger,
		backend:     backend,
		closeNotify: make(chan struct{}),

		queryTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "query_total",
			Help:        "The total number of processed queries",
			ConstLabels: lb,
		}),
		hitTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "hit_total",
			Help:        "The total number of queries that hit the cache",
			ConstLabels: lb,
		}),
		lazyHitTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "lazy_hit_total",
			Help:        "The total number of queries that hit the expired cache",
			ConstLabels: lb,
		}),
		size: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:        "size_current",
			Help:        "Current cache size in records",
			ConstLabels: lb,
		}, func() float64 {
			return float64(backend.Len())
		}),
	}

	return p, nil
}

func (c *RedisCache) Exec(ctx context.Context, qCtx *query_context.Context, next sequence.ChainWalker) error {
	c.queryTotal.Inc()
	q := qCtx.Q()

	msgKey := getMsgKey(q, c.args.Separator, c.args.Prefix)
	if len(msgKey) == 0 { // skip cache
		return next.ExecNext(ctx, qCtx)
	}

	var cachedResp *dns.Msg = nil
	if c.args.StoreOnly {
		c.logger.Debug("cache hit but store only, will query upstream and update cache", zap.Any("query", qCtx), zap.Any("resp", &cachedResp))
	} else {
		cachedResp, lazyHit := c.getRespFromCache(msgKey, c.args.LazyCacheTTL > 0 || c.args.LazyCacheTTL == redis.KeepTTL, cache_backend.ExpiredMsgTtl)
		if cachedResp != nil {
			c.hitTotal.Inc()
			if lazyHit {
				c.lazyHitTotal.Inc()
				c.logger.Debug("lazy cache hit ", zap.Any("query", qCtx), zap.Any("resp", &cachedResp))
				c.doLazyUpdate(msgKey, qCtx, next)
			} else {
				c.logger.Debug("cache hit ", zap.Any("query", qCtx), zap.Any("resp", &cachedResp))
			}
			cachedResp.Id = q.Id // change msg id
			qCtx.SetResponse(cachedResp)
		}
	}

	err := next.ExecNext(ctx, qCtx)

	if r := qCtx.R(); r != nil && cachedResp != r { // pointer compare. r is not cachedResp
		if resp := qCtx.GetBlackHoleOrigResp(); resp != nil {
			c.saveRespToCache(msgKey, resp, c.args.LazyCacheTTL, qCtx.GetBlackHoleTag())
		} else {
			c.saveRespToCache(msgKey, r, c.args.LazyCacheTTL, "")
		}
		c.updatedKey.Add(1)
	}
	return err
}
