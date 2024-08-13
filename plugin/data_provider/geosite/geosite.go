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

package geosite

import (
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/geofile"
	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/domain"
	"github.com/IrineSistiana/mosdns/v5/plugin/data_provider"
	"github.com/IrineSistiana/mosdns/v5/plugin/data_provider/domain_set"
	"github.com/xtls/xray-core/app/router"
	"runtime/debug"
	"strings"
)

const PluginType = "geosite"

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
}

func Init(bp *coremain.BP, args any) (any, error) {
	m, err := NewV2rayGeosite(bp, args.(*Args))
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (geosite V2rayGeosite) CleanUp() {
	geofile.CleanUp()
}

type Args struct {
	Files   []string `yaml:"files"`
	Domains []string `yaml:"domains"`
}

var _ data_provider.DomainMatcherProvider = (*V2rayGeosite)(nil)

type V2rayGeosite struct {
	mg []domain.Matcher[struct{}]
}

func (d *V2rayGeosite) GetDomainMatcher() domain.Matcher[struct{}] {
	return MatcherGroup(d.mg)
}

func NewV2rayGeosite(bp *coremain.BP, args *Args) (*V2rayGeosite, error) {
	v2gs := &V2rayGeosite{}

	m := domain.NewDomainMixMatcher()

	if args.Files != nil {
		for _, file := range args.Files {
			split := strings.Split(file, ":")
			path := split[0]
			code := split[1]
			if err := LoadFile(path, code, m); err != nil {
				return nil, err
			}
			if m.Len() > 0 {
				v2gs.mg = append(v2gs.mg, m)
			}
		}
	}
	if args.Domains != nil {
		err := domain_set.LoadExps(args.Domains, m)
		if err != nil {
			return nil, err
		}
	}
	return v2gs, nil
}

func LoadFile(file string, code string, m *domain.MixMatcher[struct{}]) error {
	if len(file) > 0 {
		domains, err := geofile.LoadSite(file, code)
		if err != nil {
			return err
		}
		if domains == nil || len(domains) == 0 {
			return fmt.Errorf(code + " not found in " + file)
		}
		for _, dom := range domains {
			var pattern = dom.Value
			switch dom.Type {
			case router.Domain_Full:
				pattern = domain.MatcherFull + ":" + pattern
			case router.Domain_Domain:
				pattern = domain.MatcherDomain + ":" + pattern
			case router.Domain_Regex:
				pattern = domain.MatcherRegexp + ":" + pattern
			case router.Domain_Plain:
				pattern = domain.MatcherKeyword + ":" + pattern
			default:
				continue
			}
			if err := m.Add(pattern, struct{}{}); err != nil {
				return err
			}
		}
	}
	defer debug.FreeOSMemory()
	return nil
}

type MatcherGroup []domain.Matcher[struct{}]

func (mg MatcherGroup) Match(s string) (struct{}, bool) {
	for _, m := range mg {
		if _, ok := m.Match(s); ok {
			return struct{}{}, true
		}
	}
	return struct{}{}, false
}
