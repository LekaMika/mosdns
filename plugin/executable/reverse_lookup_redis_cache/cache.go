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

package reverse_lookup_redis_cache

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
	PluginType = "reverse_lookup_redis_cache"
)

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
}

const (
	defaultLazyUpdateTimeout = time.Second * 5
	expiredMsgTtl            = 5
)

var _ sequence.RecursiveExecutable = (*ReverseLookupRedisCache)(nil)

var TagNameMap = make(map[string]*ReverseLookupRedisCache)

type Args struct {
	Url          string `yaml:"url"`
	RedisTimeout int    `yaml:"redis_timeout"`
	LazyCacheTTL int    `yaml:"lazy_cache_ttl"`

	Separator string `yaml:"separator"`
	Prefix    string `yaml:"prefix"`

	ReadOnly bool `yaml:"read_only"`
}

func (a *Args) init() {
	if &a.Separator == nil || len(a.Separator) == 0 {
		a.Separator = ":"
	}
}

type ReverseLookupRedisCache struct {
	args *Args

	logger       *zap.Logger
	backend      cache.Cache[string, string]
	lazyUpdateSF singleflight.Group
	closeOnce    sync.Once
	closeNotify  chan struct{}
	updatedKey   atomic.Uint64
}

func Init(bp *coremain.BP, args any) (any, error) {
	c, err := NewPtrRedisCache(args.(*Args), cache.RedisCacheOpts{
		Logger:     bp.L(),
		MetricsTag: bp.Tag(),
	})
	if err != nil {
		return nil, err
	}

	TagNameMap[bp.Tag()] = c
	return c, nil
}

func NewPtrRedisCache(args *Args, opts cache.RedisCacheOpts) (*ReverseLookupRedisCache, error) {
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
	rcOpts := cache.RedisCacheOpts{
		Client:        r,
		ClientCloser:  r,
		ClientTimeout: time.Duration(args.RedisTimeout) * time.Millisecond,
		Logger:        logger,
	}

	backend, err := cache.NewRedisCache(rcOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to init redis cache, %w", err)
	}
	p := &ReverseLookupRedisCache{
		args:        args,
		logger:      logger,
		backend:     backend,
		closeNotify: make(chan struct{}),
	}

	return p, nil
}

func (c *ReverseLookupRedisCache) Exec(ctx context.Context, qCtx *query_context.Context, next sequence.ChainWalker) error {
	q := qCtx.Q()
	question := q.Question[0]
	qtype := question.Qtype
	if qtype == dns.TypePTR {
		r := c.handlePtr(q)
		if r != nil {
			qCtx.SetResponse(r)
			return nil
		}
	}

	err := next.ExecNext(ctx, qCtx)

	if !c.args.ReadOnly && (qtype == dns.TypeA || qtype == dns.TypeAAAA) {
		if r := qCtx.R(); r != nil {
			c.StorePtr(q, r)
		}
	}

	return err
}

func (c *ReverseLookupRedisCache) handlePtr(q *dns.Msg) *dns.Msg {
	ptr, ok := c.GetPtr(q)
	println(ptr)
	if ok && len(ptr) > 0 {
		r := new(dns.Msg)
		setDefaultVal(r)
		r.SetReply(q)
		r.Answer = append(r.Answer, &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   q.Question[0].Name,
				Rrtype: q.Question[0].Qtype,
				Class:  q.Question[0].Qclass,
				Ttl:    5,
			},
			Ptr: ptr,
		})
		return r
	}
	return nil
}

func (c *ReverseLookupRedisCache) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeNotify)
	})
	return c.backend.Close()
}
