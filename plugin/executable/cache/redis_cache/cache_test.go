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
	"github.com/miekg/dns"
	"net/netip"
	"testing"
)

var (
	args = &Args{
		Url:          "unix:///dev/shm/redis.sock?db=1",
		LazyCacheTTL: 86400,
		Separator:    ":",
		Prefix:       "test_prefix",
		StoreOnly:    false,
	}
	cache, _ = NewRedisCache(args, "test", nil)
)

func Test_store(t *testing.T) {
	q := &dns.Msg{
		Question: make([]dns.Question, 0),
	}
	q.Question = append(q.Question, dns.Question{
		Name:   "test.xx",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	})
	addr, _ := netip.ParseAddr("127.0.0.1")
	r := &dns.Msg{}
	setDefaultVal(r)
	r.SetReply(q)
	r.Answer = append(r.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   q.Question[0].Name,
			Rrtype: q.Question[0].Qtype,
			Class:  q.Question[0].Qclass,
			Ttl:    600,
		},
		A: addr.AsSlice(),
	})
	key := getMsgKey(q, args.Separator, args.Prefix)
	ok := cache.saveRespToCache(key, r, args.LazyCacheTTL)
	println(ok)
}

func Test_get(t *testing.T) {
	q := &dns.Msg{
		Question: make([]dns.Question, 0),
	}
	q.Question = append(q.Question, dns.Question{
		Name:   "test.xx",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	})
	key := getMsgKey(q, args.Separator, args.Prefix)
	resp, ok := cache.getRespFromCache(key, true, args.LazyCacheTTL)
	if ok {
		println(string(marshalDNS(resp)))
	} else {
		println("no data")
	}
}

func Test_del(t *testing.T) {
	err := cache.Clean()
	if err != nil {
		t.Fatal("del err", err)
	}
}
