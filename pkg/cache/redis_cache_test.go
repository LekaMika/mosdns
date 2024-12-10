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

package cache

import (
	"fmt"
	"github.com/redis/go-redis/v9"
	"testing"
	"time"
)

var _ Cache[string, string] = (*RedisCache[string, string])(nil)
var _ Cache[string, string] = (Cache[string, string])((*RedisCache[string, string])(nil))

//func BenchmarkUnmarshalDNS(b *testing.B) {
//	rawBytes := `{"Resp":{"Id":6733,"Response":true,"Opcode":0,"Authoritative":false,"Truncated":false,"RecursionDesired":true,"RecursionAvailable":true,"Zero":false,"AuthenticatedData":false,"CheckingDisabled":false,"Rcode":0,"Question":[{"Name":"www.qq.com.","Qtype":1,"Qclass":1}],"Answer":[{"Hdr":{"Name":"www.qq.com.","Rrtype":5,"Class":1,"Ttl":300,"Rdlength":36},"Target":"ins-r23tsuuf.ias.tencent-cloud.net."},{"Hdr":{"Name":"ins-r23tsuuf.ias.tencent-cloud.net.","Rrtype":1,"Class":1,"Ttl":300,"Rdlength":4},"A":"61.241.54.232"},{"Hdr":{"Name":"ins-r23tsuuf.ias.tencent-cloud.net.","Rrtype":1,"Class":1,"Ttl":300,"Rdlength":4},"A":"61.241.54.211"}],"Ns":[],"Extra":[]},"StoredTime":"2024-08-09T09:28:35.365373551+08:00","ExpirationTime":"2024-08-09T09:33:35.365373551+08:00"}`
//	b.ResetTimer()
//	for i := 0; i < b.N; i++ {
//		unmarshalDNS([]byte(rawBytes))
//	}
//}

func TestRedisCache_Get(t *testing.T) {
	url := "unix:///dev/shm/redis.sock?db=1"
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(fmt.Errorf("invalid redis url, %w", err))
	}
	opt.MaxRetries = -1
	r := redis.NewClient(opt)
	opts := RedisCacheOpts{
		Client:        r,
		ClientCloser:  r,
		ClientTimeout: time.Duration(600000) * time.Millisecond,
		Logger:        nopLogger,
	}
	c, err := NewRedisCache[string, string](opts)
	if err != nil {
		t.Fatal(fmt.Errorf("invalid redis url, %w", err))
	}
	var cache Cache[string, string] = c
	v, d, ok := cache.Get("query_cache:A:IN:qq.com.")
	if !ok {
		t.Fatal(fmt.Errorf("get faild"))
	}
	fmt.Printf("%v - > %v", v, d)
}

func TestRedisCache_Store(t *testing.T) {
	url := "unix:///dev/shm/redis.sock?db=1"
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(fmt.Errorf("invalid redis url, %w", err))
	}
	opt.MaxRetries = -1
	r := redis.NewClient(opt)
	opts := RedisCacheOpts{
		Client:        r,
		ClientCloser:  r,
		ClientTimeout: time.Duration(600000) * time.Millisecond,
		Logger:        nopLogger,
	}
	c, err := NewRedisCache[string, string](opts)
	if err != nil {
		t.Fatal(fmt.Errorf("invalid redis url, %w", err))
	}
	var cache Cache[string, string] = c
	v, d, ok := cache.Get("query_cache:A:IN:qq.com.")
	if !ok {
		t.Fatal(fmt.Errorf("get faild"))
	}
	fmt.Printf("%v - > %v", v, d)

	cache.Store("query_cache:A:IN:test.qq.com.", v, 600*time.Second)
}
