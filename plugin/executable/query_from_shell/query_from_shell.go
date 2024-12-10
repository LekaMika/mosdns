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

package queryfromshell

import (
	"bytes"
	"context"
	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/dnsutils"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/miekg/dns"
	"net"
	"net/netip"
	"os/exec"
	"strings"
	"sync"
)

const PluginType = "query_from_shell"

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
	sequence.MustRegExecQuickSetup(PluginType, QuickSetup)
}

var mutex sync.Mutex
var pluginCache = make(map[string]*queryFromShell)

var _ sequence.Executable = (*queryFromShell)(nil)

type queryFromShell struct {
	args *Args
}

type Args struct {
	Cmd string `yaml:"cmd"`
}

func Init(bp *coremain.BP, args any) (any, error) {
	cmd := args.(*Args).Cmd
	return getQueryFromShellPlugin(cmd), nil
}

func QuickSetup(_ sequence.BQ, cmd string) (any, error) {
	return getQueryFromShellPlugin(cmd), nil
}

func getQueryFromShellPlugin(cmd string) *queryFromShell {
	plugin := pluginCache[cmd]
	if plugin == nil {
		mutex.Lock()
		plugin = &queryFromShell{
			args: &Args{
				Cmd: cmd,
			},
		}
		pluginCache[cmd] = plugin
		mutex.Unlock()
	}
	return plugin
}

func (b *queryFromShell) Exec(_ context.Context, qCtx *query_context.Context) error {
	if r := b.response(qCtx.Q()); r != nil {
		qCtx.SetResponse(r)
	}
	return nil
}

func (b *queryFromShell) response(m *dns.Msg) *dns.Msg {
	if len(m.Question) != 1 {
		return nil
	}
	q := m.Question[0]
	typ := q.Qtype
	fqdn := q.Name
	if q.Qclass != dns.ClassINET || (typ != dns.TypeA && typ != dns.TypeAAAA) {
		return nil
	}

	cmdLine := b.args.Cmd

	// 要执行的命令
	cmd := exec.Command("sh", "-c", cmdLine)

	// 创建一个缓冲区来捕获命令的输出
	var out bytes.Buffer
	cmd.Stdout = &out

	// 执行命令
	err := cmd.Run()
	if err != nil {
		return nil
	}

	ipv4 := make([]netip.Addr, 0)
	ipv6 := make([]netip.Addr, 0)

	result := out.String()
	ips := strings.Split(result, "\n")
	for i := range ips {
		ipStr := ips[i]
		ip := net.ParseIP(ipStr)
		if ip != nil {
			addr, ok := netip.AddrFromSlice(ip)
			if ok {
				if ip.To4() != nil {
					ipv4 = append(ipv4, addr)
				} else {
					ipv6 = append(ipv6, addr)
				}
			}
		}
	}

	r := new(dns.Msg)
	r.SetReply(m)
	switch {
	case typ == dns.TypeA && len(ipv4) > 0:
		for _, ip := range ipv4 {
			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   fqdn,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    10,
				},
				A: ip.AsSlice(),
			}
			r.Answer = append(r.Answer, rr)
		}
	case typ == dns.TypeAAAA && len(ipv6) > 0:
		for _, ip := range ipv6 {
			rr := &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   fqdn,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    10,
				},
				AAAA: ip.AsSlice(),
			}
			r.Answer = append(r.Answer, rr)
		}
	}

	// Append fake SOA record for empty reply.
	if len(r.Answer) == 0 {
		r.Ns = []dns.RR{dnsutils.FakeSOA(fqdn)}
	}
	return r
}
