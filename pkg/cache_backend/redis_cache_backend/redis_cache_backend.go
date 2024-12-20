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

package redis_cache_backend

import (
	"context"
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"sync/atomic"
	"time"
)

var backends = make(map[string]*redis.Client)

var nopLogger = zap.NewNop()

type RedisCache[K cache_backend.StringKey, V string] struct {
	addr string

	closed      atomic.Bool
	closeNotify chan struct{}

	client *redis.Client
}

func NewRedisCache[K cache_backend.StringKey, V string](addr string) (*RedisCache[K, V], error) {
	var client = backends[addr]
	if client == nil {
		opt, err := redis.ParseURL(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid redis url, %w", err)
		}
		opt.MaxRetries = -1
		client = redis.NewClient(opt)
		backends[addr] = client
	}
	return &RedisCache[K, V]{
		addr:   addr,
		client: client,
	}, nil
}

// Close closes the inner cleaner of this cache.
func (c *RedisCache[K, V]) Close() error {
	delete(backends, c.addr)
	err := c.client.Close()
	c.closed.Store(true)
	return err
}

func (c *RedisCache[K, V]) Get(key K) (value V, expirationTime time.Time, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	data, err := c.client.Get(ctx, string(key)).Result()
	if err != nil {
		if err != redis.Nil {
			nopLogger.Warn("redis get", zap.Error(err))
		}
		return V(data), time.Now(), false
	}
	duration, err1 := c.client.TTL(ctx, string(key)).Result()
	if err1 != nil {
		duration = 0
	}
	return V(data), time.Now().Add(duration * time.Second), true
}

// Store stores this kv in cache. If expirationTime is before time.Now(),
// Store is an noop.
func (c *RedisCache[K, V]) Store(key K, msg V, cacheTtl time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := c.client.Set(ctx, string(key), msg, cacheTtl).Err(); err != nil {
		nopLogger.Warn("redis set", zap.Error(err))
	}
}

// Len returns the current size of this cache.
func (c *RedisCache[K, V]) Len() int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	i, err := c.client.DBSize(ctx).Result()
	if err != nil {
		nopLogger.Error("dbsize", zap.Error(err))
		return 0
	}
	return int(i)
}

func (c *RedisCache[K, V]) Range(f func(key K, value V, expirationTime time.Time) error) error {
	return nil
}

func (c *RedisCache[K, V]) Flush() {
}

func (c *RedisCache[K, V]) Delete(key K) error {
	keys, err := c.client.Keys(context.Background(), string(key)).Result()
	if err != nil {
		return err
	}
	_, err = c.client.Del(context.Background(), keys...).Result()
	return err
}
