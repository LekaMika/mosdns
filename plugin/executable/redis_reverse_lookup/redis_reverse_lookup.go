/*
 * Copyright (C) 2020-2022, IrineSistiana
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

package reverselookup

import (
	"context"
	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/pkg/utils"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/cache"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/cache/redis_cache"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/miekg/dns"
	"net/netip"
)

const (
	PluginType = "redis_reverse_lookup"
)

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
}

var _ sequence.RecursiveExecutable = (*ReverseLookup)(nil)

type Args struct {
	HandlePTR bool   `yaml:"handle_ptr"`
	TTL       int    `yaml:"ttl"` // Default is 7200 (2h)
	CacheTag  string `yaml:"cache_tag"`
}

func (a *Args) init() {
	utils.SetDefaultUnsignNum(&a.TTL, 7200)
}

type ReverseLookup struct {
	args  *Args
	cache cache.Cache[cache_backend.StringKey, string]
}

func Init(bp *coremain.BP, args any) (any, error) {
	return NewReverseLookup(bp, args.(*Args))
}

func NewReverseLookup(bp *coremain.BP, args *Args) (any, error) {
	args.init()
	c := bp.M().GetPlugin(args.CacheTag).(*redis_cache.RedisCache)
	p := &ReverseLookup{
		args:  args,
		cache: c,
	}
	return p, nil
}

func (p *ReverseLookup) Exec(ctx context.Context, qCtx *query_context.Context, next sequence.ChainWalker) error {
	q := qCtx.Q()
	if r := p.ResponsePTR(q); r != nil {
		r.SetReply(q)
		qCtx.SetResponse(r)
		return nil
	}
	if err := next.ExecNext(ctx, qCtx); err != nil {
		return err
	}
	p.saveIPs(q, qCtx.R())
	return nil
}

func (p *ReverseLookup) Close() error {
	return p.cache.Close()
}

func (p *ReverseLookup) lookup(q *dns.Msg) *dns.Msg {
	r, _ := p.cache.QueryDns(q)
	return r
}

func (p *ReverseLookup) ResponsePTR(q *dns.Msg) *dns.Msg {
	if p.args.HandlePTR && len(q.Question) > 0 && q.Question[0].Qtype == dns.TypePTR {
		r := p.lookup(q)
		return r
	}
	return nil
}

func (p *ReverseLookup) saveIPs(q, r *dns.Msg) {
	if r == nil {
		return
	}
	p.cache.StoreDns(q, r)
}

func as16(n netip.Addr) netip.Addr {
	if n.Is6() {
		return n
	}
	return netip.AddrFrom16(n.As16())
}
