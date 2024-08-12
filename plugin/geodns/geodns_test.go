package geodns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
	"github.com/stretchr/testify/require"
)

func TestFiltering(t *testing.T) {
	ctx := context.Background()

	// locations
	Orgrimar := "4444:1::"
	LocationNotInDB := "127.0.0.1"
	WarsongHold := "4444:2::"
	Stormwind := "4444:3::"
	ThunderBluff := "4444:4::"
	dummy := "bad ip"

	for _, tc := range []struct {
		name     string
		rw       dns.ResponseWriter
		records  []string
		expected []string
		maxRec   int
		err      bool
	}{
		{
			name:     "simple udp test",
			rw:       &test.ResponseWriter{},
			records:  []string{WarsongHold, Orgrimar, Stormwind, dummy},
			expected: []string{WarsongHold},
			maxRec:   1,
		},
		{
			name:     "simple tcp test",
			rw:       &test.ResponseWriter{TCP: true, RemoteIP: ThunderBluff},
			records:  []string{Orgrimar, WarsongHold, Stormwind, dummy},
			expected: []string{Orgrimar},
			maxRec:   1,
		},
		{
			name:     "server location not in db",
			rw:       &test.ResponseWriter{TCP: true, RemoteIP: ThunderBluff},
			records:  []string{LocationNotInDB, WarsongHold, Stormwind},
			expected: []string{WarsongHold, Stormwind},
			maxRec:   2,
		},
		{
			name:     "max rec greater than records amount",
			rw:       &test.ResponseWriter{RemoteIP: ThunderBluff},
			records:  []string{Orgrimar, WarsongHold, Stormwind, LocationNotInDB},
			expected: []string{Orgrimar, WarsongHold, Stormwind, LocationNotInDB},
			maxRec:   4,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			geoDNS, err := newGeoDNS("testdata", tc.maxRec)
			require.NoError(t, err)
			geoDNS.Next = newTestHandler(map[string][]string{
				"test.neofs": tc.records,
			})

			req := new(dns.Msg)
			req.SetQuestion(dns.Fqdn("test.neofs"), dns.TypeAAAA)
			req.Question[0].Qclass = dns.ClassINET

			rec := dnstest.NewRecorder(tc.rw)
			status, err := geoDNS.ServeDNS(ctx, rec, req)
			require.NoError(t, err)
			require.Equal(t, dns.RcodeSuccess, status)

			require.Equal(t, len(tc.expected), len(rec.Msg.Answer))

			for i, expected := range tc.expected {
				res := rec.Msg.Answer[i].(*dns.AAAA).AAAA.String()
				require.Equal(t, expected, res)
			}
		})
	}
}

func TestFilteringEDNS0(t *testing.T) {
	ctx := context.Background()

	// locations
	Orgrimar := "4444:1::"
	WarsongHold := "4444:2::"
	Stormwind := "4444:3::"
	ThunderBluff := "4444:4::"
	maxRec := 1

	geoDNS, err := newGeoDNS("testdata/", maxRec)
	require.NoError(t, err)
	geoDNS.Next = newTestHandler(map[string][]string{
		"test.neofs": {Orgrimar, WarsongHold, Stormwind},
	})

	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("test.neofs"), dns.TypeAAAA)
	req.Question[0].Qclass = dns.ClassINET

	o := new(dns.OPT)
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	e := new(dns.EDNS0_SUBNET)
	e.Code = dns.EDNS0SUBNET
	e.Family = 2          // 1 for IPv4 source address, 2 for IPv6
	e.SourceNetmask = 128 // 32 for IPV4, 128 for IPv6
	e.SourceScope = 0
	e.Address = net.ParseIP("2a02:d340::") // France
	o.Option = append(o.Option, e)
	req.Extra = []dns.RR{o}

	rw := &test.ResponseWriter{RemoteIP: ThunderBluff}

	rec := dnstest.NewRecorder(rw)
	status, err := geoDNS.ServeDNS(ctx, rec, req)
	require.NoError(t, err)
	require.Equal(t, dns.RcodeSuccess, status)

	require.Equal(t, 1, len(rec.Msg.Answer))

	res := rec.Msg.Answer[0].(*dns.AAAA).AAAA.String()
	require.Equal(t, Orgrimar, res)
}

type testHandler struct {
	db map[string][]string
}

func newTestHandler(db map[string][]string) testHandler {
	return testHandler{db: db}
}

func (n testHandler) ServeDNS(_ context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	req := request.Request{Req: r, W: w}

	res, ok := n.db[strings.TrimSuffix(req.QName(), ".")]
	if !ok {
		return dns.RcodeNotZone, fmt.Errorf("not found entry: %s", req.QName())
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Answer = getTestAnswers(req, res)

	_ = w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (n testHandler) Name() string { return pluginName }

func getTestAnswers(req request.Request, res []string) []dns.RR {
	var records []dns.RR
	hdr := dns.RR_Header{Name: req.QName(), Rrtype: req.QType(), Class: req.QClass(), Ttl: 0}

	var f func(res string) dns.RR

	switch req.QType() {
	case dns.TypeA:
		f = func(data string) dns.RR {
			return &dns.A{Hdr: hdr, A: net.ParseIP(data)}
		}
	case dns.TypeAAAA:
		f = func(data string) dns.RR {
			return &dns.AAAA{Hdr: hdr, AAAA: net.ParseIP(data)}
		}
	default:
		f = func(data string) dns.RR {
			return &dns.TXT{Hdr: hdr, Txt: []string{data}}
		}
	}

	for _, data := range res {
		records = append(records, f(data))
	}

	return records
}

func TestDistance(t *testing.T) {
	var cityWithCountry geoip2.City
	cityWithCountry.Country.GeoNameID = 1
	cityWithCountry.Location.Longitude = 1
	cityWithCountry.Location.Latitude = 1

	var country geoip2.Country
	country.Country.GeoNameID = 1

	for _, tc := range []struct {
		name     string
		from     *IPInformation
		to       *IPInformation
		expected *DistanceInfo
	}{
		{
			name:     "empty",
			expected: &DistanceInfo{Distance: maxDistance},
		},
		{
			name: "match city country",
			from: &IPInformation{
				City: &cityWithCountry,
			},
			to: &IPInformation{
				Country: &country,
			},
			expected: &DistanceInfo{Distance: maxDistance, CountryMatched: true},
		},
		{
			name: "the same location",
			from: &IPInformation{
				City: &cityWithCountry,
			},
			to: &IPInformation{
				City: &cityWithCountry,
			},
			expected: &DistanceInfo{Distance: 0, CountryMatched: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := distance(tc.from, tc.to)
			require.Equal(t, tc.expected, res)
		})
	}

}
