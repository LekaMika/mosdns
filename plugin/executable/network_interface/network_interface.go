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

package network_interface

import (
	"context"
	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/miekg/dns"
	"net/netip"
	"net"
	"sync"
)

const PluginType = "network_interface"

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
	sequence.MustRegExecQuickSetup(PluginType, QuickSetup)
}

var mutex sync.Mutex
var pluginCache = make(map[string]*networkInterface)

var _ sequence.Executable = (*networkInterface)(nil)

type networkInterface struct {
	args *Args
}

type Args struct {
	InterfaceName string `yaml:"interface"`
}

func Init(bp *coremain.BP, args any) (any, error) {
    name := args.(*Args).InterfaceName
	return getNetworkInterfacePlugin(name), nil
}

func QuickSetup(_ sequence.BQ, name string) (any, error) {
    return getNetworkInterfacePlugin(name), nil
}

func getNetworkInterfacePlugin(name string) *networkInterface {
    plugin := pluginCache[name]
    if plugin == nil {
        mutex.Lock()
        plugin = &networkInterface{
            args: &Args {
                InterfaceName: name,
            },
        }
        pluginCache[name] = plugin
        mutex.Unlock()
    }
	return plugin
}

func (b *networkInterface) Exec(_ context.Context, qCtx *query_context.Context) error {
	if r := b.response(qCtx.Q()); r != nil {
		qCtx.SetResponse(r)
	}
	return nil
}

func (b *networkInterface) response(q *dns.Msg) *dns.Msg {

    name := b.args.InterfaceName

    interfaces, err := net.Interfaces()
    if err != nil {
    	return nil
    }

	ipv4s := make([]netip.Addr , 0)
	ipv6s := make([]netip.Addr , 0)

    for _, i := range interfaces {
        if i.Name != name {
            continue
        }

    	addrs, err := i.Addrs()
    	if err != nil {
    		continue
    	}

    	for _, addr := range addrs {
    		var ip net.IP
    		switch v := addr.(type) {
    		case *net.IPNet:
                ip = v.IP
    		case *net.IPAddr:
                ip = v.IP
    		}

            addr, ok := netip.AddrFromSlice(ip)
            if ok {
                if ip.To4() != nil {
                    ipv4s = append(ipv4s, addr)
                } else {
                    ipv6s = append(ipv6s, addr)
                }
            }
        }
    }

	if len(q.Question) != 1 {
		return nil
	}

	qName := q.Question[0].Name
	qtype := q.Question[0].Qtype

	switch {
	case qtype == dns.TypeA && len(ipv4s) > 0:
		r := new(dns.Msg)
		r.SetReply(q)
		for _, addr := range ipv4s {
			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   qName,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				A: addr.AsSlice(),
			}
			r.Answer = append(r.Answer, rr)
		}
		return r

	case qtype == dns.TypeAAAA && len(ipv6s) > 0:
		r := new(dns.Msg)
		r.SetReply(q)
		for _, addr := range ipv6s {
			rr := &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   qName,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				AAAA: addr.AsSlice(),
			}
			r.Answer = append(r.Answer, rr)
		}
		return r
	}
	return nil
}
