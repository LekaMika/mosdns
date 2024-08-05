/*
 * Copyright (C) 2023, VizV
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

package v2ray_geoip

import (
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/netlist"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"net/netip"
	"os"
	"strings"

	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/plugin/data_provider"
	_ "github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"google.golang.org/protobuf/proto"
)

const PluginType = "v2ray_geoip"

var geoipListFiles = map[string]*routercommon.GeoIPList{}

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
	Sets  []string `yaml:"sets"`
	Files []string `yaml:"files"`
	Codes []string `yaml:"codes"`
}

var _ data_provider.IPMatcherProvider = (*V2rayGeoip)(nil)

type V2rayGeoip struct {
	mg []netlist.Matcher
}

func (d *V2rayGeoip) GetIPMatcher() netlist.Matcher {
	return MatcherGroup(d.mg)
}

// NewV2rayGeoip inits a V2rayGeoip from given args.
func NewV2rayGeoip(bp *coremain.BP, args *Args) (*V2rayGeoip, error) {
	v2gs := &V2rayGeoip{}

	l := netlist.NewList()

	cs := map[string]bool{}
	for _, code := range args.Codes {
		cs[strings.ToUpper(code)] = true
	}

	if err := LoadFiles(args.Files, cs, l); err != nil {
		return nil, err
	}
	if l.Len() > 0 {
		l.Sort()
		v2gs.mg = append(v2gs.mg, l)
	}

	for _, tag := range args.Sets {
		provider, _ := bp.M().GetPlugin(tag).(data_provider.IPMatcherProvider)
		if provider == nil {
			return nil, fmt.Errorf("%s is not a IPMatcherProvider", tag)
		}
		m := provider.GetIPMatcher()
		v2gs.mg = append(v2gs.mg, m)
	}
	return v2gs, nil
}

func LoadFiles(fs []string, cs map[string]bool, l *netlist.List) error {
	for i, f := range fs {
		if err := LoadFile(f, cs, l); err != nil {
			return fmt.Errorf("failed to load file #%d %s, %w", i, f, err)
		}
	}
	return nil
}

func LoadFile(f string, cs map[string]bool, l *netlist.List) error {
	if len(f) > 0 {
		var geoipList = geoipListFiles[f]
		if geoipList == nil {
			geoipList = &routercommon.GeoIPList{}
			data, err := os.ReadFile(f)
			if err != nil {
				return err
			}
			if err := proto.Unmarshal(data, geoipList); err != nil {
				return err
			}
			geoipListFiles[f] = geoipList
		}

		if err := loadFromGeoip(l, geoipList, cs); err != nil {
			return err
		}
	}
	return nil
}

func loadFromGeoip(l *netlist.List, geoipList *routercommon.GeoIPList, cs map[string]bool) error {
	for _, entry := range geoipList.Entry {
		if !cs[entry.CountryCode] {
			continue
		}

		for i, cidr := range entry.Cidr {
			ip, ok := netip.AddrFromSlice(cidr.Ip)
			if !ok {
				return fmt.Errorf("invalid ip at index #%d, %s", i, cidr.Ip)
			}
			prefix, err := ip.Prefix(int(cidr.Prefix))
			if !ok {
				return fmt.Errorf("invalid prefix at index #%d, %w", i, err)
			}
			l.Append(prefix)
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
