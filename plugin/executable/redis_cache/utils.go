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
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/pkg/redis_cache"
	"time"

	"github.com/IrineSistiana/mosdns/v5/pkg/dnsutils"
	"github.com/miekg/dns"
)

// getMsgKey returns a string key for the query msg, or an empty
// string if query should not be cached.
func getMsgKey(q *dns.Msg, separator string) string {
	question := q.Question[0]
	return fmt.Sprintf("%s%s%s%s%s", dns.TypeToString[question.Qtype], separator, dns.ClassToString[question.Qclass], separator, question.Name)
}

func setDefaultVal(m *dns.Msg) *dns.Msg {
	if m == nil {
		return nil
	}

	if m.Answer == nil {
		m.Answer = make([]dns.RR, 0)
	}
	if m.Ns == nil {
		m.Ns = make([]dns.RR, 0)
	}
	if m.Extra == nil {
		m.Extra = make([]dns.RR, 0)
	}

	return m
}

// getRespFromCache returns the cached response from cache.
// The ttl of returned msg will be changed properly.
// Returned bool indicates whether this response is hit by lazy cache.
// Note: Caller SHOULD change the msg id because it's not same as query's.
func getRespFromCache(key string, backend *redis_cache.Cache, lazyCacheEnabled bool, lazyTtl int) (*dns.Msg, bool) {
	// Lookup cache
	item, ok := backend.Get(key)

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

// saveRespToCache saves r to cache backend. It returns false if r
// should not be cached and was skipped.
func saveRespToCache(msgKey string, r *dns.Msg, backend *redis_cache.Cache, lazyCacheTtl int) bool {
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
			const maxEmtpyAnswerTtl = 300
			msgTtl = time.Duration(min(minTTL, maxEmtpyAnswerTtl)) * time.Second
			cacheTtl = msgTtl
		} else {
			msgTtl = time.Duration(minTTL) * time.Second
			if lazyCacheTtl > 0 {
				cacheTtl = time.Duration(lazyCacheTtl) * time.Second
			} else {
				cacheTtl = msgTtl
			}
		}
	}
	if msgTtl <= 0 || cacheTtl <= 0 {
		return false
	}

	now := time.Now()
	expirationTime := now.Add(msgTtl)
	v := &redis_cache.Item{
		Resp:           setDefaultVal(r),
		StoredTime:     now,
		ExpirationTime: expirationTime,
	}
	backend.Store(msgKey, v, cacheTtl)
	return true
}
