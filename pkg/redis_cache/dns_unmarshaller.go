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

package redis_cache

import (
	"fmt"
	"log"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/miekg/dns"
	"github.com/savaki/jq"
)

func stringToRR(rawJSON []byte, rtype uint16) (rr dns.RR, err error) {
	switch rtype {
	case dns.TypeNone:
		rr = &dns.NULL{}
	case dns.TypeA:
		rr = &dns.A{}
	case dns.TypeNS:
		rr = &dns.NS{}
	case dns.TypeMD:
		rr = &dns.MD{}
	case dns.TypeMF:
		rr = &dns.MF{}
	case dns.TypeCNAME:
		rr = &dns.CNAME{}
	case dns.TypeSOA:
		rr = &dns.SOA{}
	case dns.TypeMB:
		rr = &dns.MB{}
	case dns.TypeMG:
		rr = &dns.MG{}
	case dns.TypeMR:
		rr = &dns.MR{}
	case dns.TypeNULL:
		rr = &dns.NULL{}
	case dns.TypePTR:
		rr = &dns.PTR{}
	case dns.TypeHINFO:
		rr = &dns.HINFO{}
	case dns.TypeMINFO:
		rr = &dns.MINFO{}
	case dns.TypeMX:
		rr = &dns.MX{}
	case dns.TypeTXT:
		rr = &dns.TXT{}
	case dns.TypeRP:
		rr = &dns.RP{}
	case dns.TypeAFSDB:
		rr = &dns.AFSDB{}
	case dns.TypeX25:
		rr = &dns.X25{}
	case dns.TypeISDN:
		rr = &dns.NULL{} // not implemented
	case dns.TypeRT:
		rr = &dns.RT{}
	case dns.TypeNSAPPTR:
		rr = &dns.NSAPPTR{}
	case dns.TypeSIG:
		rr = &dns.SIG{}
	case dns.TypeKEY:
		rr = &dns.KEY{}
	case dns.TypePX:
		rr = &dns.PX{}
	case dns.TypeGPOS:
		rr = &dns.GPOS{}
	case dns.TypeAAAA:
		rr = &dns.AAAA{}
	case dns.TypeLOC:
		rr = &dns.LOC{}
	case dns.TypeNXT:
		rr = &dns.NULL{} // not implemented
	case dns.TypeEID:
		rr = &dns.EID{}
	case dns.TypeNIMLOC:
		rr = &dns.NIMLOC{}
	case dns.TypeSRV:
		rr = &dns.SRV{}
	case dns.TypeATMA:
		rr = &dns.NULL{} // not implemented
	case dns.TypeNAPTR:
		rr = &dns.NAPTR{}
	case dns.TypeKX:
		rr = &dns.KX{}
	case dns.TypeCERT:
		rr = &dns.CERT{}
	case dns.TypeDNAME:
		rr = &dns.DNAME{}
	case dns.TypeOPT:
		rr = &dns.OPT{}
	case dns.TypeAPL:
		rr = &dns.APL{}
	case dns.TypeDS:
		rr = &dns.DS{}
	case dns.TypeSSHFP:
		rr = &dns.SSHFP{}
	case dns.TypeRRSIG:
		rr = &dns.RRSIG{}
	case dns.TypeNSEC:
		rr = &dns.NSEC{}
	case dns.TypeDNSKEY:
		rr = &dns.DNSKEY{}
	case dns.TypeDHCID:
		rr = &dns.DHCID{}
	case dns.TypeNSEC3:
		rr = &dns.NSEC3{}
	case dns.TypeNSEC3PARAM:
		rr = &dns.NSEC3PARAM{}
	case dns.TypeTLSA:
		rr = &dns.TLSA{}
	case dns.TypeSMIMEA:
		rr = &dns.SMIMEA{}
	case dns.TypeHIP:
		rr = &dns.HIP{}
	case dns.TypeNINFO:
		rr = &dns.NINFO{}
	case dns.TypeRKEY:
		rr = &dns.RKEY{}
	case dns.TypeTALINK:
		rr = &dns.TALINK{}
	case dns.TypeCDS:
		rr = &dns.CDS{}
	case dns.TypeCDNSKEY:
		rr = &dns.CDNSKEY{}
	case dns.TypeOPENPGPKEY:
		rr = &dns.OPENPGPKEY{}
	case dns.TypeCSYNC:
		rr = &dns.CSYNC{}
	case dns.TypeZONEMD:
		rr = &dns.ZONEMD{}
	case dns.TypeSVCB:
		rr = &dns.SVCB{}
	case dns.TypeHTTPS:
		rr = &dns.HTTPS{}
	case dns.TypeSPF:
		rr = &dns.SPF{}
	case dns.TypeUINFO:
		rr = &dns.UINFO{}
	case dns.TypeUID:
		rr = &dns.UID{}
	case dns.TypeGID:
		rr = &dns.GID{}
	case dns.TypeUNSPEC:
		rr = &dns.NULL{} // not implemented
	case dns.TypeNID:
		rr = &dns.NID{}
	case dns.TypeL32:
		rr = &dns.L32{}
	case dns.TypeL64:
		rr = &dns.L64{}
	case dns.TypeLP:
		rr = &dns.LP{}
	case dns.TypeEUI48:
		rr = &dns.EUI48{}
	case dns.TypeEUI64:
		rr = &dns.EUI64{}
	case dns.TypeURI:
		rr = &dns.URI{}
	case dns.TypeCAA:
		rr = &dns.CAA{}
	case dns.TypeAVC:
		rr = &dns.AVC{}

	default:
		return nil, fmt.Errorf("unknown rtype %d", rtype)
	}

	err = json.Unmarshal(rawJSON, &rr)
	return
}

type trimmedHdr struct {
	Rrtype uint16 `json:"Rrtype"`
}
type trimmedJSON struct {
	Answer []struct {
		Hdr trimmedHdr `json:"Hdr"`
	} `json:"Answer"`
	Ns []struct {
		Hdr trimmedHdr `json:"Hdr"`
	} `json:"Ns"`
	Extra []struct {
		Hdr trimmedHdr `json:"Hdr"`
	} `json:"Extra"`
}

type itemJson struct {
	Resp           trimmedJSON `json:"Resp"`
	StoredTime     *time.Time
	ExpirationTime *time.Time
}

func UnmarshalDNS(rawBytes []byte) (msg dns.Msg) {

	if err := json.Unmarshal([]byte(rawBytes), &msg); err != nil {
		// an error message is expected here since Answer, Ns and Extra are interfaces and it fails to unmarshal them
		msg.Answer = []dns.RR{}
		msg.Ns = []dns.RR{}
		msg.Extra = []dns.RR{}
	}
	// trimmed JSON is used to grab the RRtypes out of the raw message
	trimmed := trimmedJSON{}
	if err := json.Unmarshal([]byte(rawBytes), &trimmed); err != nil {
		log.Println(err)
	}

	for i, rr := range trimmed.Answer {
		query, _ := jq.Parse(fmt.Sprintf(".Answer.[%d]", i))
		if iter, err := query.Apply([]byte(rawBytes)); err != nil {
			log.Println(err)
		} else {
			if rr, err := stringToRR(iter, rr.Hdr.Rrtype); err != nil {
				log.Println(err)
			} else {
				msg.Answer = append(msg.Answer, rr)
			}
		}
	}
	for i, rr := range trimmed.Ns {
		query, _ := jq.Parse(fmt.Sprintf(".Ns.[%d]", i))
		if iter, err := query.Apply([]byte(rawBytes)); err != nil {
			log.Println(err)
		} else {
			if rr, err := stringToRR(iter, rr.Hdr.Rrtype); err != nil {
				log.Println(err)
			} else {
				msg.Ns = append(msg.Ns, rr)
			}
		}
	}
	for i, rr := range trimmed.Extra {
		query, _ := jq.Parse(fmt.Sprintf(".Extra.[%d]", i))
		if iter, err := query.Apply([]byte(rawBytes)); err != nil {
			log.Println(err)
		} else {
			if rr, err := stringToRR(iter, rr.Hdr.Rrtype); err != nil {
				log.Println(err)
			} else {
				msg.Extra = append(msg.Extra, rr)
			}
		}
	}
	return
}

func UnmarshalDNSItem(rawBytes []byte) (item Item) {

	if err := json.Unmarshal([]byte(rawBytes), &item); err != nil {
		// an error message is expected here since Answer, Ns and Extra are interfaces and it fails to unmarshal them
		item.Resp.Answer = []dns.RR{}
		item.Resp.Ns = []dns.RR{}
		item.Resp.Extra = []dns.RR{}
	}
	// trimmed JSON is used to grab the RRtypes out of the raw message
	trimmed := itemJson{}
	if err := json.Unmarshal(rawBytes, &trimmed); err != nil {
		log.Println(err)
	}
	item.StoredTime = *trimmed.StoredTime
	item.ExpirationTime = *trimmed.ExpirationTime

	for i, rr := range trimmed.Resp.Answer {
		query, _ := jq.Parse(fmt.Sprintf(".Resp.Answer.[%d]", i))
		if iter, err := query.Apply(rawBytes); err != nil {
			log.Println(err)
		} else {
			if rr, err := stringToRR(iter, rr.Hdr.Rrtype); err != nil {
				log.Println(err)
			} else {
				item.Resp.Answer = append(item.Resp.Answer, rr)
			}
		}
	}
	for i, rr := range trimmed.Resp.Ns {
		query, _ := jq.Parse(fmt.Sprintf(".Resp.Ns.[%d]", i))
		if iter, err := query.Apply(rawBytes); err != nil {
			log.Println(err)
		} else {
			if rr, err := stringToRR(iter, rr.Hdr.Rrtype); err != nil {
				log.Println(err)
			} else {
				item.Resp.Ns = append(item.Resp.Ns, rr)
			}
		}
	}
	for i, rr := range trimmed.Resp.Extra {
		query, _ := jq.Parse(fmt.Sprintf(".Resp.Extra.[%d]", i))
		if iter, err := query.Apply(rawBytes); err != nil {
			log.Println(err)
		} else {
			if rr, err := stringToRR(iter, rr.Hdr.Rrtype); err != nil {
				log.Println(err)
			} else {
				item.Resp.Extra = append(item.Resp.Extra, rr)
			}
		}
	}
	return
}
