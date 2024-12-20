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
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend"
	"github.com/miekg/dns"
	"strings"
)

// getMsgKey returns a string key for the query msg, or an empty
// string if query should not be cached.
func getMsgKey(addr string, separator string, prefix string) cache_backend.StringKey {
	if len(strings.TrimSpace(prefix)) > 0 {
		return cache_backend.StringKey(fmt.Sprintf("%s%s%s", prefix, separator, addr))
	} else {
		return cache_backend.StringKey(fmt.Sprintf("%s", addr))
	}
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

//func (c *ReverseLookupRedisCache) GetPtr(q *dns.Msg) (string, bool) {
//	addr, err := dnsutils.ParsePTRQName(q.Question[0].Name)
//	if err != nil {
//		return "", false
//	}
//	if !(addr.IsValid() && (addr.Is4() || addr.Is6())) {
//		return "", false
//	}
//
//	ptrKey := getMsgKey(addr.String(), c.args.Separator, c.args.Prefix)
//	value, _, ok := c.backend.Get(cache_backend.StringKey(ptrKey))
//	if !ok {
//		return "", false
//	}
//	return string(value), true
//}
//
//func (c *ReverseLookupRedisCache) StorePtrKeyPair(name string, ip string, expires time.Time) {
//	now := time.Now()
//	if expires.Before(now) {
//		return
//	}
//	ptrKey := getMsgKey(ip, c.args.Separator, c.args.Prefix)
//	c.backend.Store(cache_backend.StringKey(ptrKey), name, expires.Sub(now))
//}
