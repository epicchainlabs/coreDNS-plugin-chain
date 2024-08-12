package healthchecker

import (
	"github.com/miekg/dns"
)

type (
	ResponseWriter struct {
		dns.ResponseWriter
		filter *HealthCheckFilter
	}
)

func NewResponseWriter(w dns.ResponseWriter, filter *HealthCheckFilter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		filter:         filter,
	}
}

func (r *ResponseWriter) WriteMsg(res *dns.Msg) error {
	qName := res.Question[0].Name

	if len(res.Answer) == 0 {
		log.Debugf("answer is empty, nothing to do")
		return r.ResponseWriter.WriteMsg(res)
	}

	res.Answer = r.filter.FilterRecords(res.Answer)
	if len(res.Answer) == 0 {
		log.Warningf("no answer returned: couldn't resolve %s: no healthy IPs", qName)
	}

	return r.ResponseWriter.WriteMsg(res)
}

func isSupportedType(qtype uint16) bool {
	return qtype == dns.TypeA || qtype == dns.TypeAAAA
}
