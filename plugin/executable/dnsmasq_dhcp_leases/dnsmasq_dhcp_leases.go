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
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend"
	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/domain"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/cache"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/cache/redis_cache"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"os"
	"strings"
)

const PluginType = "dnsmasq_dhcp_leases"

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
}

var _ sequence.Executable = (*Leases)(nil)

type Args struct {
	File     string   `yaml:"file"`
	Suffixs  []string `yaml:"suffix"`
	CacheTag string   `yaml:"cache_tag"`
}

type Leases struct {
	args       *Args
	logger     *zap.Logger
	file       string
	leases     []*dnsmasq.Lease
	ipv4Leases []*dnsmasq.Lease
	ipv6Leases []*dnsmasq.Lease
	leaseChan  chan []*dnsmasq.Lease
	matcher    domain.Matcher[*leasesGroup]
	cache      cache.Cache[cache_backend.StringKey, string]
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
		args:      args,
		logger:    bp.L(),
		file:      args.File,
		leases:    readLeases,
		leaseChan: leases,
	}

	if len(strings.TrimSpace(args.CacheTag)) > 0 {
		redisCache := bp.M().GetPlugin(args.CacheTag).(*redis_cache.RedisCache)
		l.cache = redisCache
	}

	l.buildMatchers()
	go l.start()
	return l, nil
}

func (l *Leases) start() {
	go dnsmasq.WatchLeases(context.Background(), l.file, l.leaseChan)
	for leaseBatch := range l.leaseChan {
		newLeases := make([]*dnsmasq.Lease, 0)
		for _, lease := range leaseBatch {
			newLeases = append(newLeases, lease)
		}
		l.leases = newLeases
		l.buildMatchers()
	}
}

func (l *Leases) buildMatchers() {
	leases := l.leases
	ipMap := make(map[string]*leasesGroup)
	//if l.cache != nil {
	//	l.cache.StorePtrKeyPair(hostname, ipAddr.String(), -1)
	//}
	l.ipv4Leases = make([]*dnsmasq.Lease, 0)
	l.ipv6Leases = make([]*dnsmasq.Lease, 0)
	for _, lease := range leases {
		hostname := lease.Hostname
		ipAddr := lease.IPAddr
		//expires := lease.Expires
		if !ipAddr.IsValid() {
			continue
		}
		if hostname == "*" {
			continue
		}
		key := hostname + "."
		ips := ipMap[key]
		if ips == nil {
			ips = &leasesGroup{
				ipv4Leases: make([]*dnsmasq.Lease, 0),
				ipv6Leases: make([]*dnsmasq.Lease, 0),
			}
			ipMap[key] = ips
			for i2 := range l.args.Suffixs {
				suffix := l.args.Suffixs[i2]
				ipMap[key+suffix+"."] = ips
			}
		}

		if ipAddr.Is4() {
			ips.ipv4Leases = append(ips.ipv4Leases, lease)
			l.ipv4Leases = append(l.ipv4Leases, lease)
		} else if ipAddr.Is6() {
			ips.ipv6Leases = append(ips.ipv6Leases, lease)
			l.ipv6Leases = append(l.ipv6Leases, lease)
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
	l.cache.Clean()
	for fqdn := range ipMap {
		l.saveCache(fqdn, dns.TypeA)
		l.saveCache(fqdn, dns.TypeAAAA)
	}
	for _, lease := range l.leases {
		addr := lease.IPAddr
		l.savePtr2Cache(addr)
	}
}

func (l *Leases) lookup(fqdn string) (ipv4, ipv6 []*dnsmasq.Lease) {
	ips, ok := l.matcher.Match(fqdn)
	if !ok {
		return nil, nil // no such host
	}
	return ips.ipv4Leases, ips.ipv6Leases
}

func (l *Leases) Exec(ctx context.Context, qCtx *query_context.Context) error {
	if qCtx.R() == nil {
		if r := l.responsePtr(qCtx.Q()); r != nil {
			l.logger.Info("dhcp ptr cache hit", zap.Any("query", qCtx), zap.Any("resp", r))
			qCtx.SetResponse(r)
		}
	}
	if qCtx.R() == nil {
		if r := l.responseQuery(qCtx.Q()); r != nil {
			l.logger.Info("dhcp cache hit", zap.Any("query", qCtx), zap.Any("resp", r))
			qCtx.SetResponse(r)
		}
	}
	return nil
}
