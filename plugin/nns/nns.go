package nns

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/nns/contract"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/transfer"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/nspcc-dev/neofs-contract/nns"
)

type NNS struct {
	Next      plugin.Handler
	Contracts []*contract.Contract
	Log       clog.P
	dnsDomain string
}

type Records struct {
	Name string
	Type nns.RecordType
	Data []string
}

const dot = "."

// ServeDNS implements the plugin.Handler interface.
// This method gets called when example is used in a Server.
func (n NNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	res, err := n.resolveRecords(request.Request{W: w, Req: r})
	if err != nil {
		n.Log.Warning(err)
		return plugin.NextOrFailure(n.Name(), n.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Answer = res

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (n NNS) Name() string { return pluginName }

// Transfer implements the transfer.Transfer interface.
// Don't use serial because we support only AXFR.
func (n NNS) Transfer(zone string, serial uint32) (<-chan []dns.RR, error) {
	trimmedZone := n.Contracts[0].PrepareName(zone, n.dnsDomain)
	records, err := n.Contracts[0].GetRecords(trimmedZone, nns.RecordType(dns.TypeSOA))
	if err != nil {
		n.Log.Warningf("couldn't transfer zone '%s' as '%s': %s", zone, trimmedZone, err.Error())
		return nil, transfer.ErrNotAuthoritative
	}
	if len(records) == 0 {
		return nil, transfer.ErrNotAuthoritative
	}

	ch := make(chan []dns.RR)
	go func() {
		defer close(ch)

		recs, err := n.zoneTransfers(zone)
		if err != nil {
			n.Log.Warningf("couldn't transfer zone '%s': %s", zone, err.Error())
			return
		}

		ch <- recs
	}()

	return ch, nil
}

func (n *NNS) setDNSDomain(name string) {
	n.dnsDomain = strings.Trim(name, dot)
}

func (n NNS) resolveRecords(state request.Request) ([]dns.RR, error) {
	var err error
	var result []dns.RR

	for _, nnsContract := range n.Contracts {
		result, err = n.resolveContractRecords(nnsContract, state)
		if err != nil {
			n.Log.Warningf("resolve in contract '%s': %s", nnsContract.Hash().StringLE(), err.Error())
		}
	}

	return result, nil
}

func (n NNS) resolveContractRecords(nnsContract *contract.Contract, state request.Request) ([]dns.RR, error) {
	name := nnsContract.PrepareName(state.QName(), n.dnsDomain)

	nnsType, err := getNNSType(state)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve '%s' (type %d) as '%s': %w",
			state.QName(), state.QType(), name, err)
	}

	resolved, err := nnsContract.Resolve(name, nnsType)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve '%s' (type %d) as '%s': %w",
			state.QName(), state.QType(), name, err)
	}

	hdr := dns.RR_Header{Name: state.QName(), Rrtype: state.QType(), Class: state.QClass(), Ttl: 0}
	res, err := formResRecords(hdr, resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve '%s' (type %d) as '%s': %w",
			state.QName(), state.QType(), name, err)
	}

	return res, nil
}

func (n NNS) zoneTransfers(zone string) ([]dns.RR, error) {
	result := make(map[string]*Records)
	for i, nnsContract := range n.Contracts {
		transferRecords, err := n.allTransferRecords(nnsContract, zone, i == 0)
		if err != nil {
			n.Log.Warningf("get all records in contract '%s': %s", nnsContract.Hash().StringLE(), err.Error())
		}

		for key, recs := range transferRecords {
			result[key] = recs
		}
	}

	return formZoneTransfer(result)
}

func (n NNS) allTransferRecords(nnsContract *contract.Contract, zone string, needSOA bool) (map[string]*Records, error) {
	name := nnsContract.PrepareName(zone, n.dnsDomain)

	records, err := nnsContract.GetAllRecords(name)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*Records)
	for _, record := range records {
		updatedName := appendRoot(record.Name)
		key := recKey(updatedName, record.Type)
		recs := result[key]
		if recs == nil {
			recs = &Records{
				Name: updatedName,
				Type: record.Type,
				Data: []string{},
			}
		}
		recs.Data = append(recs.Data, record.Data)
		result[key] = recs
	}

	if needSOA {
		soaRecs := result[recKey(appendRoot(name), nns.RecordType(dns.TypeSOA))]
		if len(soaRecs.Data) != 1 {
			return nil, fmt.Errorf("invalid number of soa records: %d", len(soaRecs.Data))
		}
	}

	return result, nil
}

func recKey(name string, recType nns.RecordType) string {
	return name + strconv.Itoa(int(recType))
}

func formZoneTransfer(recordsMap map[string]*Records) ([]dns.RR, error) {
	if len(recordsMap) == 0 {
		return nil, fmt.Errorf("records must not be empty")
	}

	records := make([]*Records, 0, len(recordsMap))
	for _, rec := range recordsMap {
		records = append(records, rec)
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Name == records[j].Name {
			return records[i].Type < records[j].Type
		}
		return records[i].Name < records[j].Name
	})

	var err error
	var soaRecord *dns.SOA
	results := make([]dns.RR, 1, len(records))

	for _, recs := range records {
		if recs.Type == nns.RecordType(dns.TypeSOA) {
			soaRecord, err = formSoaRecord(recs)
			if err != nil {
				return nil, err
			}
			continue
		}

		for _, data := range recs.Data {
			rec, err := formRec(uint16(recs.Type), data, dns.RR_Header{
				Name:   recs.Name,
				Rrtype: uint16(recs.Type),
				Class:  dns.ClassINET,
			})
			if err != nil {
				return nil, err
			}
			results = append(results, rec)
		}
	}

	results[0] = soaRecord
	results = append(results, soaRecord)

	return results, nil
}

func formSoaRecord(rec *Records) (*dns.SOA, error) {
	if rec.Type != nns.RecordType(dns.TypeSOA) {
		return nil, fmt.Errorf("invalid type for soa record")
	}

	if len(rec.Data) != 1 {
		return nil, fmt.Errorf("invalid len for soa record")
	}

	split := strings.Split(rec.Data[0], " ")
	if len(split) != 7 {
		return nil, fmt.Errorf("invalid soa record: %s", rec.Data[0])
	}

	name := appendRoot(split[0])
	if rec.Name != name {
		return nil, fmt.Errorf("invalid soa record, mismatched names: %s %s", rec.Name, name)
	}

	lenSerial := len(split[2])
	if lenSerial > 10 { // timestamp with second precision
		lenSerial = 10
	}
	serial, err := parseUint32(split[2][:lenSerial])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid serial: %s", split[2])
	}
	refresh, err := parseUint32(split[3])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid refresh: %s", split[3])
	}
	retry, err := parseUint32(split[4])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid retry: %s", split[4])
	}
	expire, err := parseUint32(split[5])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid expire: %s", split[5])
	}
	ttl, err := parseUint32(split[6])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid ttl: %s", split[6])
	}

	return &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns:      name,
		Mbox:    strings.ReplaceAll(appendRoot(split[1]), "@", "."),
		Serial:  serial,
		Refresh: refresh,
		Retry:   retry,
		Expire:  expire,
		Minttl:  ttl,
	}, nil
}

func appendRoot(data string) string {
	if len(data) > 0 && data[len(data)-1] != '.' {
		return data + "."
	}
	return data
}

func parseUint32(data string) (uint32, error) {
	parsed, err := strconv.ParseUint(data, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(parsed), nil
}

func getNNSType(req request.Request) (nns.RecordType, error) {
	switch req.QType() {
	case dns.TypeTXT:
		return nns.TXT, nil
	case dns.TypeA:
		return nns.A, nil
	case dns.TypeAAAA:
		return nns.AAAA, nil
	case dns.TypeCNAME:
		return nns.CNAME, nil
	}
	return 0, fmt.Errorf("usupported record type: %s", dns.Type(req.QType()))
}

func formResRecords(hdr dns.RR_Header, resolved []string) ([]dns.RR, error) {
	var records []dns.RR
	for _, record := range resolved {
		rec, err := formRec(hdr.Rrtype, record, hdr)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

func formRec(reqType uint16, res string, hdr dns.RR_Header) (dns.RR, error) {
	switch reqType {
	case dns.TypeTXT:
		return &dns.TXT{Hdr: hdr, Txt: []string{res}}, nil
	case dns.TypeA:
		return &dns.A{Hdr: hdr, A: net.ParseIP(res)}, nil
	case dns.TypeAAAA:
		return &dns.AAAA{Hdr: hdr, AAAA: net.ParseIP(res)}, nil
	case dns.TypeCNAME:
		return &dns.CNAME{Hdr: hdr, Target: res}, nil
	}

	return nil, fmt.Errorf("usupported record type: %s", dns.Type(reqType))
}
