package geodns

import (
	"fmt"
	"math"
	"net"
	"sort"

	"github.com/golang/geo/s2"
	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
)

const maxDistance float64 = 360

var emptyLocation geoip2.City

type recordInfo struct {
	endpoint     string
	record       dns.RR
	distanceInfo *DistanceInfo
}

func (r *recordInfo) String() string {
	return r.record.String()
}

// ResponseFilter is a type of ResponseWriter that captures all messages written to it.
type ResponseFilter struct {
	dns.ResponseWriter
	filter *filter
	client net.IP
}

// NewResponseFilter makes and returns a new response filter.
func NewResponseFilter(w dns.ResponseWriter, filter *filter, client net.IP) *ResponseFilter {
	return &ResponseFilter{
		ResponseWriter: w,
		filter:         filter,
		client:         client,
	}
}

// WriteMsg records the message and its length written to it and call the
// underlying ResponseWriter's WriteMsg method.
func (r *ResponseFilter) WriteMsg(res *dns.Msg) error {
	if len(res.Answer) == 0 {
		log.Debugf("answer is empty, nothing to do")
		return r.ResponseWriter.WriteMsg(res)
	}

	clientInf := r.filter.db.IPInfo(r.client)
	if clientInf.IsEmpty() {
		log.Warningf(formErrMessage(r.client))
		if r.filter.maxRecords < len(res.Answer) {
			res.Answer = res.Answer[:r.filter.maxRecords]
		}
		return r.ResponseWriter.WriteMsg(res)
	}

	recInfos := make([]recordInfo, 0, len(res.Answer))

	for _, rec := range res.Answer {
		endpoint := getEndpointFromRecord(rec)
		if endpoint == "" {
			log.Warningf("couldn't get an endpoint: wrong record type: %s", rec.String())
			continue
		}
		var distInfo *DistanceInfo
		serverInf := r.filter.db.IPInfo(net.ParseIP(endpoint))
		if serverInf.IsEmpty() {
			log.Debugf(formErrMessage(rec))
			distInfo = &DistanceInfo{Distance: maxDistance}
		} else {
			distInfo = distance(clientInf, serverInf)
		}
		recInfos = append(recInfos, recordInfo{endpoint: endpoint, record: rec, distanceInfo: distInfo})
	}

	res.Answer = chooseClosest(recInfos, r.filter.maxRecords)
	return r.ResponseWriter.WriteMsg(res)
}

func getEndpointFromRecord(record dns.RR) (endpoint string) {
	if aRec, ok := record.(*dns.A); ok {
		endpoint = aRec.A.String()
	} else if aaaaRec, ok := record.(*dns.AAAA); ok {
		endpoint = aaaaRec.AAAA.String()
	}
	return
}

// Write is a wrapper that records the length of the messages that get written to it.
func (r *ResponseFilter) Write(buf []byte) (int, error) {
	return r.ResponseWriter.Write(buf)
}

func isSupportedType(qtype uint16) bool {
	return qtype == dns.TypeA || qtype == dns.TypeAAAA
}

func distance(from, to *IPInformation) *DistanceInfo {
	res := &DistanceInfo{Distance: maxDistance}
	if from == nil || to == nil {
		return res
	}

	var fromCountry, toCountry uint
	fromLocation, toLocation := emptyLocation.Location, emptyLocation.Location
	if from.City != nil {
		fromCountry = from.City.Country.GeoNameID
		fromLocation = from.City.Location
	}
	if to.City != nil {
		toCountry = to.City.Country.GeoNameID
		toLocation = to.City.Location
	}

	if fromLocation != emptyLocation.Location && toLocation != emptyLocation.Location {
		ll1 := s2.LatLngFromDegrees(fromLocation.Latitude, fromLocation.Longitude)
		ll2 := s2.LatLngFromDegrees(toLocation.Latitude, toLocation.Longitude)
		angle := ll1.Distance(ll2)
		res.Distance = math.Abs(angle.Degrees())
	}

	if fromCountry == 0 && from.Country != nil {
		fromCountry = from.Country.Country.GeoNameID
	}
	if toCountry == 0 && to.Country != nil {
		toCountry = to.Country.Country.GeoNameID
	}
	res.CountryMatched = fromCountry == toCountry

	return res
}

func chooseClosest(recInfos []recordInfo, max int) []dns.RR {
	if len(recInfos) < max {
		max = len(recInfos)
	}

	sort.Slice(recInfos, func(i, j int) bool {
		di1 := recInfos[i].distanceInfo
		di2 := recInfos[j].distanceInfo

		if di1.Distance == maxDistance && di2.Distance == maxDistance {
			return di1.CountryMatched
		}

		return di1.Distance < di2.Distance
	})

	results := make([]dns.RR, max)
	for i := 0; i < max; i++ {
		results[i] = recInfos[i].record
	}

	return results
}

func formErrMessage(data fmt.Stringer) string {
	return fmt.Sprintf("couldn't get location %s from db: not found", data)
}
