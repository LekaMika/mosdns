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

package netlist

import (
	"bytes"
	"net/netip"
	"testing"
)

func TestIPNetList_Sort_And_Merge(t *testing.T) {
	raw := `
192.168.0.0/32 # merged
192.168.0.0/24 # merged
192.168.0.0/16
192.168.1.1/24 # merged
192.168.9.24/24 # merged
192.168.3.0/24 # merged
192.169.0.0/16
104.16.0.0/12
`
	ipNetList := NewList()
	err := LoadFromReader(ipNetList, bytes.NewBufferString(raw))
	if err != nil {
		t.Fatal(err)
	}
	ipNetList.Sort()

	if ipNetList.Len() != 3 {
		t.Fatalf("unexpected length %d", ipNetList.Len())
	}

	tests := []struct {
		name   string
		testIP netip.Addr
		want   bool
	}{
		{"0", netip.MustParseAddr("192.167.255.255"), false},
		{"1", netip.MustParseAddr("192.168.0.0"), true},
		{"2", netip.MustParseAddr("192.168.1.1"), true},
		{"3", netip.MustParseAddr("192.168.9.255"), true},
		{"4", netip.MustParseAddr("192.168.255.255"), true},
		{"5", netip.MustParseAddr("192.169.1.1"), true},
		{"6", netip.MustParseAddr("192.170.1.1"), false},
		{"7", netip.MustParseAddr("1.1.1.1"), false},
		{"8", netip.MustParseAddr("104.16.67.38"), true},
		{"9", netip.MustParseAddr("104.32.67.38"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ipNetList.Match(tt.testIP); got != tt.want {
				t.Errorf("IPNetList.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIPNetList_New_And_Contains(t *testing.T) {
	raw := `
# comment line
1.0.0.0/24 additional strings should be ignored 
2.0.0.0/23 # comment
3.0.0.0

2000:0000::/32
2000:2000::1
`

	ipNetList := NewList()
	err := LoadFromReader(ipNetList, bytes.NewBufferString(raw))
	if err != nil {
		t.Fatal(err)
	}
	ipNetList.Sort()

	tests := []struct {
		name   string
		testIP netip.Addr
		want   bool
	}{
		{"", netip.MustParseAddr("1.0.0.0"), true},
		{"", netip.MustParseAddr("1.0.0.1"), true},
		{"", netip.MustParseAddr("1.0.1.0"), false},
		{"", netip.MustParseAddr("2.0.0.0"), true},
		{"", netip.MustParseAddr("2.0.1.255"), true},
		{"", netip.MustParseAddr("2.0.2.0"), false},
		{"", netip.MustParseAddr("3.0.0.0"), true},
		{"", netip.MustParseAddr("2000:0000::"), true},
		{"", netip.MustParseAddr("2000:0000::1"), true},
		{"", netip.MustParseAddr("2000:0000:1::"), true},
		{"", netip.MustParseAddr("2000:0001::"), false},
		{"", netip.MustParseAddr("2000:2000::1"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ipNetList.Match(tt.testIP); got != tt.want {
				t.Errorf("IPNetList.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Contains(t *testing.T) {
	raw := `
173.245.48.0/20
103.21.244.0/22
103.22.200.0/22
103.31.4.0/22
141.101.64.0/18
108.162.192.0/18
190.93.240.0/20
188.114.96.0/20
197.234.240.0/22
198.41.128.0/17
162.158.0.0/15
104.16.0.0/13
104.24.0.0/14
172.64.0.0/13
131.0.72.0/22
`

	ipNetList := NewList()
	err := LoadFromReader(ipNetList, bytes.NewBufferString(raw))
	if err != nil {
		t.Fatal(err)
	}
	ipNetList.Sort()

	tests := []struct {
		name   string
		testIP netip.Addr
		want   bool
	}{
		{"", netip.MustParseAddr("104.21.51.61"), true},
	}
	for _, tt := range tests {
		match := ipNetList.Match(tt.testIP)
		t.Run(tt.name, func(tx *testing.T) {
			if got := ipNetList.Match(tt.testIP); got != tt.want {
				t.Errorf("IPNetList.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}
