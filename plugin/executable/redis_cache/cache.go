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
	"github.com/IrineSistiana/mosdns/v5/pkg/cache"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

const (
	PluginType = "redis_cache"
)

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
}

const (
	defaultLazyUpdateTimeout = time.Second * 5
	expiredMsgTtl            = 5
)

var _ sequence.RecursiveExecutable = (*RedisCache)(nil)

var backends = make(map[string]cache.Cache[string, string])

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
	backend      cache.Cache[string, string]
	lazyUpdateSF singleflight.Group
	closeOnce    sync.Once
	closeNotify  chan struct{}
	updatedKey   atomic.Uint64
}

type Item struct {
	Resp           *dns.Msg
	StoredTime     time.Time
	ExpirationTime time.Time
}

func Init(bp *coremain.BP, args any) (any, error) {
	c, err := NewRedisCache(args.(*Args), cache.RedisCacheOpts{
		Logger:     bp.L(),
		MetricsTag: bp.Tag(),
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func NewRedisCache(args *Args, opts cache.RedisCacheOpts) (*RedisCache, error) {
	args.init()

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	// serial initialization
	backend := backends[args.Url]
	if backend == nil {
		opt, err := redis.ParseURL(args.Url)
		if err != nil {
			return nil, fmt.Errorf("invalid redis url, %w", err)
		}
		opt.MaxRetries = -1
		r := redis.NewClient(opt)
		rcOpts := cache.RedisCacheOpts{
			Client:        r,
			ClientCloser:  r,
			ClientTimeout: time.Duration(args.RedisTimeout) * time.Millisecond,
			Logger:        logger,
		}
		backend, err = cache.NewRedisCache(rcOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to init redis cache, %w", err)
		}
	}
	p := &RedisCache{
		args:        args,
		logger:      logger,
		backend:     backend,
		closeNotify: make(chan struct{}),
	}
	backends[args.Url] = backend

	return p, nil
}

func (c *RedisCache) Exec(ctx context.Context, qCtx *query_context.Context, next sequence.ChainWalker) error {
	q := qCtx.Q()

	msgKey := getMsgKey(q, c.args.Separator, c.args.Prefix)
	if len(msgKey) == 0 { // skip cache
		return next.ExecNext(ctx, qCtx)
	}

	var cachedResp *dns.Msg = nil
	if c.args.StoreOnly {
		c.logger.Debug("cache hit but store only, will query upstream and update cache", zap.Any("query", qCtx), zap.Any("resp", &cachedResp))
	} else {
		cachedResp, lazyHit := getRespFromCache(msgKey, c.backend, c.args.LazyCacheTTL > 0 || c.args.LazyCacheTTL == redis.KeepTTL, expiredMsgTtl)
		if cachedResp != nil {
			if lazyHit {
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
		saveRespToCache(msgKey, r, c.backend, c.args.LazyCacheTTL)
		c.updatedKey.Add(1)
	}
	return err
}

// doLazyUpdate starts a new goroutine to execute next node and update the cache in the background.
// It has an inner singleflight.Group to de-duplicate same msgKey.
func (c *RedisCache) doLazyUpdate(msgKey string, qCtx *query_context.Context, next sequence.ChainWalker) {
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

func (c *RedisCache) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeNotify)
	})
	return c.backend.Close()
}
