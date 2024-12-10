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

package dnsmasq_dhcp_leases

import (
	"context"
	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/dnsutils"
	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/domain"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/redis_cache"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/reverse_lookup_redis_cache"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
	"github.com/miekg/dns"
	"os"
	"strings"
	"time"
)

const PluginType = "dnsmasq_dhcp_leases"

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
}

var _ sequence.Executable = (*Leases)(nil)

type Args struct {
	File                  string   `yaml:"file"`
	Suffixs               []string `yaml:"suffix"`
	CacheTag              string   `yaml:"init_cache_tag"`
	ReverseLookupCacheTag string   `yaml:"init_reverse_lookup_cache_tag"`
}

type Leases struct {
	file               string
	leases             []*dnsmasq.Lease
	ipv4Leases         []*dnsmasq.Lease
	ipv6Leases         []*dnsmasq.Lease
	leaseChan          chan []*dnsmasq.Lease
	matcher            domain.Matcher[*leasesGroup]
	cache              *redis_cache.RedisCache
	reverseLookupCache *reverse_lookup_redis_cache.ReverseLookupRedisCache
}

type leasesGroup struct {
	ipv4Leases []*dnsmasq.Lease
	ipv6Leases []*dnsmasq.Lease
}

func Init(bp *coremain.BP, args any) (any, error) {
	return NewLeases(bp, args.(*Args))
}

func NewLeases(bp *coremain.BP, args *Args) (*Leases, error) {
	leases := make(chan []*dnsmasq.Lease)
	file, err := os.Open(args.File)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	readLeases, err := dnsmasq.ReadLeases(file)
	if err != nil {
		return nil, err
	}

	l := &Leases{
		file:      args.File,
		leases:    readLeases,
		leaseChan: leases,
	}

	if len(strings.TrimSpace(args.CacheTag)) > 0 {
		redisCache := bp.M().GetPlugin(args.CacheTag).(*redis_cache.RedisCache)
		l.cache = redisCache
	}

	if len(strings.TrimSpace(args.ReverseLookupCacheTag)) > 0 {
		ptrCache := bp.M().GetPlugin(args.ReverseLookupCacheTag).(*reverse_lookup_redis_cache.ReverseLookupRedisCache)
		l.reverseLookupCache = ptrCache
	}

	l.buildMatchers(args)
	go l.start(args)
	return l, nil
}

func (l *Leases) start(args *Args) {
	go dnsmasq.WatchLeases(context.Background(), l.file, l.leaseChan)
	for leaseBatch := range l.leaseChan {
		newLeases := make([]*dnsmasq.Lease, 0)
		for _, lease := range leaseBatch {
			newLeases = append(newLeases, lease)
		}
		l.leases = newLeases
		l.buildMatchers(args)
	}
}

func (l *Leases) buildMatchers(args *Args) {
	leases := l.leases
	ipMap := make(map[string]*leasesGroup)
	//if l.cache != nil {
	//	l.cache.StorePtrKeyPair(hostname, ipAddr.String(), -1)
	//}
	l.ipv4Leases = make([]*dnsmasq.Lease, 0)
	l.ipv6Leases = make([]*dnsmasq.Lease, 0)
	for i := range leases {
		lease := leases[i]
		hostname := lease.Hostname
		ipAddr := lease.IPAddr
		expires := lease.Expires
		if !ipAddr.IsValid() {
			continue
		}
		if hostname == "*" {
			continue
		}
		ips := ipMap[hostname]
		if ips == nil {
			ips = &leasesGroup{
				ipv4Leases: make([]*dnsmasq.Lease, 0),
				ipv6Leases: make([]*dnsmasq.Lease, 0),
			}
			ipMap[hostname+"."] = ips
		}
		if ipAddr.Is4() {
			ips.ipv4Leases = append(ips.ipv4Leases, lease)
			l.ipv4Leases = append(l.ipv4Leases, lease)
		} else if ipAddr.Is6() {
			ips.ipv6Leases = append(ips.ipv6Leases, lease)
			l.ipv6Leases = append(l.ipv4Leases, lease)
		}
		if l.reverseLookupCache != nil {
			l.reverseLookupCache.StorePtrKeyPair(hostname+".", ipAddr.String(), expires)
		}
	}
	m := domain.NewMixMatcher[*leasesGroup]()
	m.SetDefaultMatcher(domain.MatcherFull)
	for key := range ipMap {
		value := ipMap[key]
		m.Add(key, value)
	}
	l.matcher = m

	// init cache data
	for key := range ipMap {
		questions := make([]dns.Question, 0)
		questions = append(questions, dns.Question{
			Name:   key,
			Qclass: dns.ClassINET,
			Qtype:  dns.TypeA,
		})
		q := &dns.Msg{
			Question: questions,
		}
		r := l.responseQuery(q)
		l.cache.Store(q, r)

		questions = make([]dns.Question, 0)
		questions = append(questions, dns.Question{
			Name:   key,
			Qclass: dns.ClassINET,
			Qtype:  dns.TypeAAAA,
		})
		q = &dns.Msg{
			Question: questions,
		}
		r = l.responseQuery(q)
		l.cache.Store(q, r)
	}
}

func (l *Leases) lookup(fqdn string) (ipv4, ipv6 []*dnsmasq.Lease) {
	ips, ok := l.matcher.Match(fqdn)
	if !ok {
		return nil, nil // no such host
	}
	return ips.ipv4Leases, ips.ipv6Leases
}

func (l *Leases) responsePtr(m *dns.Msg) *dns.Msg {
	if len(m.Question) != 1 {
		return nil
	}
	q := m.Question[0]
	typ := q.Qtype
	qcl := q.Qclass
	fqdn := q.Name
	if q.Qclass != dns.ClassINET || typ != dns.TypePTR {
		return nil
	}

	addr, _ := dnsutils.ParsePTRQName(fqdn)
	if !addr.IsValid() {
		return nil
	}
	var name string
	if addr.Is4() && len(l.ipv4Leases) > 0 {
		for i := range l.ipv4Leases {
			lease := l.ipv4Leases[i]
			ipAddr := lease.IPAddr
			if ipAddr.Compare(addr) == 0 {
				name = lease.Hostname
				break
			}
		}
	} else if addr.Is6() && len(l.ipv6Leases) > 0 {
		for i := range l.ipv6Leases {
			lease := l.ipv6Leases[i]
			ipAddr := lease.IPAddr
			if ipAddr.Compare(addr) == 0 {
				name = lease.Hostname
				break
			}
		}
	}
	if len(name) > 0 {
		r := new(dns.Msg)
		r.SetReply(m)
		r.Answer = append(r.Answer, &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   fqdn,
				Rrtype: typ,
				Class:  qcl,
				Ttl:    5,
			},
			Ptr: name + ".",
		})
		return r
	}
	return nil
}

func (l *Leases) responseQuery(m *dns.Msg) *dns.Msg {
	if len(m.Question) != 1 {
		return nil
	}
	q := m.Question[0]
	typ := q.Qtype
	fqdn := q.Name
	if q.Qclass != dns.ClassINET || (typ != dns.TypeA && typ != dns.TypeAAAA) {
		return nil
	}

	ipv4, ipv6 := l.lookup(fqdn)
	if len(ipv4)+len(ipv6) == 0 {
		return nil // no such host
	}

	now := time.Now()
	r := new(dns.Msg)
	r.SetReply(m)
	switch {
	case typ == dns.TypeA && len(ipv4) > 0:
		for _, lease := range ipv4 {
			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   fqdn,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(lease.Expires.Sub(now).Seconds()),
				},
				A: lease.IPAddr.AsSlice(),
			}
			r.Answer = append(r.Answer, rr)
		}
	case typ == dns.TypeAAAA && len(ipv6) > 0:
		for _, lease := range ipv6 {
			rr := &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   fqdn,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    uint32(lease.Expires.Sub(now).Seconds()),
				},
				AAAA: lease.IPAddr.AsSlice(),
			}
			r.Answer = append(r.Answer, rr)
		}
	}

	// Append fake SOA record for empty reply.
	if len(r.Answer) == 0 {
		r.Ns = []dns.RR{dnsutils.FakeSOA(fqdn)}
	} else {
		r.Authoritative = true
	}
	return r
}

func (l *Leases) Exec(ctx context.Context, qCtx *query_context.Context) error {
	if qCtx.R() == nil {
		if r := l.responsePtr(qCtx.Q()); r != nil {
			qCtx.SetResponse(r)
		}
	}
	if qCtx.R() == nil {
		if r := l.responseQuery(qCtx.Q()); r != nil {
			qCtx.SetResponse(r)
		}
	}
	return nil
}
