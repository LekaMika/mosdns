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

package geoip

import (
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/geofile"
	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/netlist"
	"github.com/IrineSistiana/mosdns/v5/plugin/data_provider"
	"github.com/IrineSistiana/mosdns/v5/plugin/data_provider/ip_set"
	"github.com/xtls/xray-core/common/errors"
	"net/netip"
	"strings"
)

const PluginType = "geoip"

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
}

func Init(bp *coremain.BP, args any) (any, error) {
	m, err := NewV2rayGeoip(bp, args.(*Args))
	if err != nil {
		return nil, err
	}
	return m, nil
}

type Args struct {
	Files []string `yaml:"files"`
	Ips   []string `yaml:"ips"`
}

var _ data_provider.IPMatcherProvider = (*V2rayGeoip)(nil)

type V2rayGeoip struct {
	mg []netlist.Matcher
}

func (d *V2rayGeoip) GetIPMatcher() netlist.Matcher {
	return MatcherGroup(d.mg)
}

func NewV2rayGeoip(bp *coremain.BP, args *Args) (*V2rayGeoip, error) {
	v2gs := &V2rayGeoip{}

	l := netlist.NewList()

	if args.Files != nil {
		for _, file := range args.Files {
			split := strings.Split(file, ":")
			file := split[0]
			code := split[1]
			if err := LoadFile(file, code, l); err != nil {
				return nil, err
			}
			if l.Len() > 0 {
				l.Sort()
				v2gs.mg = append(v2gs.mg, l)
			}
		}
	}
	if args.Ips != nil {
		err := ip_set.LoadFromIPs(args.Ips, l)
		if err != nil {
			return nil, err
		}
	}
	return v2gs, nil
}

func LoadFile(file string, code string, l *netlist.List) error {
	if len(file) > 0 {
		key := file + ":" + code
		ipList := geofile.IpStringCache[key]
		if ipList != nil {
			l.AppendList(ipList)
		} else {
			cidrs, err := geofile.LoadIP(file, code)
			if err != nil {
				return err
			}
			if cidrs == nil || len(cidrs) == 0 {
				return errors.New(code + " not found in " + file)
			}
			tmpList := netlist.NewList()
			for i, cidr := range cidrs {
				ip, ok := netip.AddrFromSlice(cidr.Ip)
				if !ok {
					return fmt.Errorf("invalid ip at index #%d, %s", i, cidr.Ip)
				}
				prefix, err := ip.Prefix(int(cidr.Prefix))
				if err != nil {
					return fmt.Errorf("invalid prefix at index #%d, %w", i, err)
				}
				tmpList.Append(prefix)
			}
			l.AppendList(tmpList)
			geofile.IpStringCache[key] = tmpList
		}
	}
	return nil
}

type MatcherGroup []netlist.Matcher

func (mg MatcherGroup) Match(addr netip.Addr) bool {
	for _, m := range mg {
		if m.Match(addr) {
			return true
		}
	}
	return false
}
