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

package cache

import (
	"github.com/miekg/dns"
	"time"
)

type Cache[K interface{}, V interface{}] interface {
	Get(key K) V
	Store(key K, value V, ttl time.Duration)

	QueryDns(q *dns.Msg) (*dns.Msg, bool)
	StoreDns(q *dns.Msg, r *dns.Msg)

	Close() error
	Clean() error
}

type Item struct {
	Resp           *dns.Msg
	BlockHoleTag   string
	StoredTime     time.Time
	ExpirationTime time.Time
}
