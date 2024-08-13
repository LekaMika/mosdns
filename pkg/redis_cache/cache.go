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
	"github.com/IrineSistiana/mosdns/v5/pkg/utils"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"io"
	"sync/atomic"
	"time"
)

var nopLogger = zap.NewNop()

const (
	defaultCleanerInterval = time.Second * 10
)

type Item struct {
	Resp           *dns.Msg
	StoredTime     time.Time
	ExpirationTime time.Time
}

// Cache is a simple map cache that stores values in memory.
// It is safe for concurrent use.
type Cache struct {
	Opts Opts

	Closed      atomic.Bool
	CloseNotify chan struct{}
}

type Opts struct {
	MetricsTag      string
	Size            int
	CleanerInterval time.Duration

	// Client cannot be nil.
	Client redis.Cmdable

	// ClientCloser closes Client when RedisCache.Close is called.
	// Optional.
	ClientCloser io.Closer

	// ClientTimeout specifies the timeout for read and write operations.
	// Default is 50ms.
	ClientTimeout time.Duration

	// Logger is the *zap.Logger for this RedisCache.
	// A nil Logger will disable logging.
	Logger *zap.Logger
}

func (opts *Opts) init() error {
	utils.SetDefaultNum(&opts.Size, 1024)
	utils.SetDefaultNum(&opts.CleanerInterval, defaultCleanerInterval)
	if opts.Client == nil {
		return fmt.Errorf("nil client")
	}
	utils.SetDefaultNum(&opts.ClientTimeout, time.Second)
	if opts.Logger == nil {
		opts.Logger = nopLogger
	}
	return nil
}

// New initializes a Cache.
// The minimum size is 1024.
// cleanerInterval specifies the interval that Cache scans
// and discards expired values. If cleanerInterval <= 0, a default
// interval will be used.
func New(opts Opts) (*Cache, error) {
	if err := opts.init(); err != nil {
		return nil, err
	}
	return &Cache{
		Opts: opts,
	}, nil
}

// Close closes the inner cleaner of this cache.
func (c *Cache) Close() error {
	if f := c.Opts.ClientCloser; f != nil {
		return f.Close()
	}
	return nil
}

func (c *Cache) Get(key string) (*Item, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Opts.ClientTimeout)
	defer cancel()
	str, err := c.Opts.Client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			c.Opts.Logger.Warn("redis get", zap.Error(err))
		}
		return nil, false
	}
	item := unmarshalDNSItemFromJson([]byte(str))
	return item, true
}

// Store stores this kv in cache. If expirationTime is before time.Now(),
// Store is an noop.
func (c *Cache) Store(key string, item *Item, cacheTtl time.Duration) {
	msg := marshalDNSItemToJson(*item)
	ctx, cancel := context.WithTimeout(context.Background(), c.Opts.ClientTimeout)
	defer cancel()
	if err := c.Opts.Client.Set(ctx, key, msg, cacheTtl).Err(); err != nil {
		c.Opts.Logger.Warn("redis set", zap.Error(err))
		//c.disableClient()
	}
}

// Len returns the current size of this cache.
func (c *Cache) Len() int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	i, err := c.Opts.Client.DBSize(ctx).Result()
	if err != nil {
		c.Opts.Logger.Error("dbsize", zap.Error(err))
		return 0
	}
	return int(i)
}
