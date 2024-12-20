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
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/cache"
	"github.com/miekg/dns"
	"strings"
)

var _ cache.Cache[cache_backend.StringKey, string] = (*RedisCache)(nil)

// getMsgKey returns a string key for the query msg, or an empty
// string if query should not be cached.
func getMsgKey(q *dns.Msg, separator string, prefix string) string {
	question := q.Question[0]
	if len(strings.TrimSpace(prefix)) > 0 {
		return fmt.Sprintf("%s%s%s%s%s%s%s", prefix, separator, dns.TypeToString[question.Qtype], separator, dns.ClassToString[question.Qclass], separator, question.Name)
	} else {
		return fmt.Sprintf("%s%s%s%s%s", dns.TypeToString[question.Qtype], separator, dns.ClassToString[question.Qclass], separator, question.Name)
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
