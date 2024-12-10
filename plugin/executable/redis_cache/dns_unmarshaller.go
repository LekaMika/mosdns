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
	json "github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/miekg/dns"
	"time"
)

const layout = "2006-01-02T15:04:05.999999999Z07:00"

func stringToRR(rtype uint16) (rr dns.RR, err error) {
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
		err = fmt.Errorf("unknown rtype %d", rtype)
		return
	}
	return
}

func unmarshalDNS(rawBytes []byte) *dns.Msg {

	msg := &dns.Msg{}
	root, _ := json.Get(rawBytes)
	resolve(root, msg)
	return msg
}

func unmarshalDNSItemFromJson(rawBytes []byte) *Item {

	root, _ := json.Get(rawBytes)
	storedTimeStr, _ := root.Get("StoredTime").String()
	expirationTimeStr, _ := root.Get("ExpirationTime").String()
	storedTime, _ := time.Parse(layout, storedTimeStr)
	expirationTime, _ := time.Parse(layout, expirationTimeStr)

	item := Item{}
	item.Resp = &dns.Msg{}
	item.StoredTime = storedTime
	item.ExpirationTime = expirationTime

	resp := root.GetByPath("Resp")
	resolve(*resp, item.Resp)
	return &item
}

func marshalDNSItemToJson(item Item) (r []byte) {

	r, _ = json.Marshal(item)
	return
}

func resolve(root ast.Node, msg *dns.Msg) {
	// Answer
	if nodes, err := root.GetByPath("Answer").ArrayUseNode(); err == nil {
		for _, node := range nodes {
			msg.Answer = resolveNode(&node, msg.Answer)
		}
	}
	root.GetByPath("Extra").ForEach(func(path ast.Sequence, node *ast.Node) bool {
		return true
	})
	// Ns
	if nodes, err := root.GetByPath("Ns").ArrayUseNode(); err == nil {
		for _, node := range nodes {
			msg.Ns = resolveNode(&node, msg.Ns)
		}
	}
	// Extra
	if nodes, err := root.GetByPath("Extra").ArrayUseNode(); err == nil {
		for _, node := range nodes {
			msg.Extra = resolveNode(&node, msg.Extra)
		}
	}
}

func resolveNode(node *ast.Node, result []dns.RR) []dns.RR {
	rrtype, _ := node.GetByPath("Hdr", "Rrtype").Int64()
	rr, _ := stringToRR(uint16(rrtype))
	marshal, _ := node.MarshalJSON()
	_ = json.Unmarshal(marshal, rr)
	return append(result, rr)
}
