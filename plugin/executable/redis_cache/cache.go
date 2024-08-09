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
	"github.com/IrineSistiana/mosdns/v5/pkg/redis_cache"
	"github.com/redis/go-redis/v9"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

const (
	PluginType = "redis_cache"
)

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
	sequence.MustRegExecQuickSetup(PluginType, quickSetupCache)
}

const (
	defaultLazyUpdateTimeout = time.Second * 5
	expiredMsgTtl            = 5
)

var _ sequence.RecursiveExecutable = (*Cache)(nil)

type Args struct {
	Size         int    `yaml:"size"`
	Url          string `yaml:"url"`
	RedisTimeout int    `yaml:"redis_timeout"`
	LazyCacheTTL int    `yaml:"lazy_cache_ttl"`
	Separator    string `yaml:"separator"`
	//LazyCacheReplyTTL int    `yaml:"lazy_cache_reply_ttl"`
	//CacheEverything   bool   `yaml:"cache_everything"`
	//CompressResp      bool   `yaml:"compress_resp"`
	//WhenHit           string `yaml:"when_hit"`
}

func (a *Args) init() {
	if &a.Separator == nil || len(a.Separator) == 0 {
		a.Separator = ":"
	}
}

type Cache struct {
	args *Args

	logger       *zap.Logger
	backend      *redis_cache.Cache
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
	c, err := NewCache(args.(*Args), redis_cache.Opts{
		Logger:     bp.L(),
		MetricsTag: bp.Tag(),
	})
	if err != nil {
		return nil, err
	}

	if err := c.RegMetricsTo(prometheus.WrapRegistererWithPrefix(PluginType+"_", bp.M().GetMetricsReg())); err != nil {
		return nil, fmt.Errorf("failed to register metrics, %w", err)
	}
	return c, nil
}

// QuickSetup format: [size]
// default is 1024. If size is < 1024, 1024 will be used.
func quickSetupCache(bq sequence.BQ, s string) (any, error) {
	size := 0
	if len(s) > 0 {
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("invalid size, %w", err)
		}
		size = i
	}
	// Don't register metrics in quick setup.
	return NewCache(&Args{Size: size}, redis_cache.Opts{Logger: bq.L()})
}

func NewCache(args *Args, opts redis_cache.Opts) (*Cache, error) {
	args.init()

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	opt, err := redis.ParseURL(args.Url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url, %w", err)
	}
	opt.MaxRetries = -1
	r := redis.NewClient(opt)
	rcOpts := redis_cache.Opts{
		Client:        r,
		ClientCloser:  r,
		ClientTimeout: time.Duration(args.RedisTimeout) * time.Millisecond,
		Logger:        logger,
	}

	backend, err := redis_cache.New(rcOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis cache, %w", err)
	}
	lb := map[string]string{"tag": opts.MetricsTag}
	p := &Cache{
		args:        args,
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

func (c *Cache) RegMetricsTo(r prometheus.Registerer) error {
	for _, collector := range [...]prometheus.Collector{c.queryTotal, c.hitTotal, c.lazyHitTotal, c.size} {
		if err := r.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) Exec(ctx context.Context, qCtx *query_context.Context, next sequence.ChainWalker) error {
	c.queryTotal.Inc()
	q := qCtx.Q()

	msgKey := getMsgKey(q, c.args.Separator)
	if len(msgKey) == 0 { // skip cache
		return next.ExecNext(ctx, qCtx)
	}

	cachedResp, lazyHit := getRespFromCache(msgKey, c.backend, c.args.LazyCacheTTL > 0, expiredMsgTtl)
	if cachedResp != nil {
		if lazyHit {
			c.lazyHitTotal.Inc()
			c.logger.Debug("lazy cache hit ", zap.Any("query", qCtx), zap.Any("resp", &cachedResp))
			c.doLazyUpdate(msgKey, qCtx, next)
		} else {
			c.logger.Debug("cache hit ", zap.Any("query", qCtx), zap.Any("resp", &cachedResp))
		}
		c.hitTotal.Inc()
		cachedResp.Id = q.Id // change msg id
		qCtx.SetResponse(cachedResp)
	}

	err := next.ExecNext(ctx, qCtx)

	if r := qCtx.R(); r != nil && cachedResp != r { // pointer compare. r is not cachedResp
		saveRespToCache(msgKey, r, c.backend, c.args.LazyCacheTTL)
		c.updatedKey.Add(1)
	}
	return err
}

// doLazyUpdate starts a new goroutine to execute next node and update the cache in the background.
// It has an inner singleflight.Group to de-duplicate same msgKey.
func (c *Cache) doLazyUpdate(msgKey string, qCtx *query_context.Context, next sequence.ChainWalker) {
	qCtxCopy := qCtx.Copy()
	lazyUpdateFunc := func() (any, error) {
		defer c.lazyUpdateSF.Forget(msgKey)
		qCtx := qCtxCopy

		c.logger.Debug("start lazy cache update", qCtx.InfoField())
		ctx, cancel := context.WithTimeout(context.Background(), defaultLazyUpdateTimeout)
		defer cancel()

		err := next.ExecNext(ctx, qCtx)
		if err != nil {
			c.logger.Warn("failed to update lazy cache", qCtx.InfoField(), zap.Error(err))
		}

		r := qCtx.R()
		if r != nil {
			saveRespToCache(msgKey, r, c.backend, c.args.LazyCacheTTL)
			c.updatedKey.Add(1)
		}
		c.logger.Debug("lazy cache updated", qCtx.InfoField())
		return nil, nil
	}
	c.lazyUpdateSF.DoChan(msgKey, lazyUpdateFunc) // DoChan won't block this goroutine
}

func (c *Cache) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeNotify)
	})
	return c.backend.Close()
}
