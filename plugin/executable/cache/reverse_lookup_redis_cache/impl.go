package reverse_lookup_redis_cache

import (
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/pkg/cache_backend"
	"github.com/IrineSistiana/mosdns/v5/pkg/dnsutils"
	"github.com/miekg/dns"
	"net"
	"strings"
	"time"
)

func (c *ReverseLookupRedisCache) Get(key cache_backend.StringKey) string {
	//value, _, _ := c.backend.Get(key)
	//return value
	addr, err := dnsutils.ParsePTRQName(string(key))
	if err != nil {
		return ""
	}
	if !(addr.IsValid() && (addr.Is4() || addr.Is6())) {
		return ""
	}

	ptrKey := getMsgKey(addr.String(), c.args.Separator, c.args.Prefix)
	value, _, ok := c.backend.Get(ptrKey)
	if !ok {
		return ""
	}
	return value
}

func (c *ReverseLookupRedisCache) Store(key cache_backend.StringKey, value string, ttl time.Duration) {
	msgKey := getMsgKey(string(key), c.args.Separator, c.args.Prefix)
	c.backend.Store(msgKey, value, ttl)
}

func (c *ReverseLookupRedisCache) QueryDns(q *dns.Msg) (*dns.Msg, bool) {
	ptr := c.Get(cache_backend.StringKey(q.Question[0].Name))
	if len(ptr) > 0 {
		r := new(dns.Msg)
		setDefaultVal(r)
		r.SetReply(q)
		r.Answer = append(r.Answer, &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   q.Question[0].Name,
				Rrtype: q.Question[0].Qtype,
				Class:  q.Question[0].Qclass,
				Ttl:    5,
			},
			Ptr: ptr,
		})
		return r, true
	}
	return nil, false
}

func (c *ReverseLookupRedisCache) StoreDns(q *dns.Msg, r *dns.Msg) {
	for i := range r.Answer {
		rr := r.Answer[i]
		var ip net.IP
		switch rr := rr.(type) {
		case *dns.A:
			ip = rr.A
		case *dns.AAAA:
			ip = rr.AAAA
		default:
			continue
		}

		addr := ip.String()
		question := q.Question[0]
		name := question.Name
		minTTL := dnsutils.GetMinimalTTL(r)
		ptrKey := getMsgKey(addr, c.args.Separator, c.args.Prefix)

		c.backend.Store(ptrKey, name, time.Duration(minTTL)*time.Second)
	}
}

func (c *ReverseLookupRedisCache) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeNotify)
	})
	return c.backend.Close()
}

func (c *ReverseLookupRedisCache) Clean() error {
	if len(strings.TrimSpace(c.args.Prefix)) > 0 && len(strings.TrimSpace(c.args.Separator)) > 0 {
		return c.backend.Delete(cache_backend.StringKey(fmt.Sprintf("%s%s*", c.args.Prefix, c.args.Separator)))
	}
	return nil
}
