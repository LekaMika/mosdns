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

package geofile

import (
	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/netlist"
	"github.com/xtls/xray-core/app/router"
	"runtime/debug"
)

var (
	fileCache = make(map[string][]byte)
	IPCache   = make(map[string]*router.GeoIP)
	SiteCache = make(map[string]*router.GeoSite)

	IpStringCache   = make(map[string]*netlist.List)
	SiteStringCache = make(map[string][]string)
)

func Release() {
	fileCache = make(map[string][]byte)
	IPCache = make(map[string]*router.GeoIP)
	SiteCache = make(map[string]*router.GeoSite)

	IpStringCache = make(map[string]*netlist.List)
	SiteStringCache = make(map[string][]string)
	defer debug.FreeOSMemory()
}

func readAssetByCache(file string) ([]byte, error) {
	fileBytes := fileCache[file]
	if fileBytes != nil {
		return fileBytes, nil
	}
	bytes, err := readFile(file)
	if err != nil {
		return nil, err
	}
	fileCache[file] = fileBytes
	return bytes, err
}
