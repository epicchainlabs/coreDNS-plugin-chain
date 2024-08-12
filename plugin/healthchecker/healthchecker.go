package healthchecker

import (
	"context"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
)

const pluginName = "healthchecker"

var log = clog.NewWithPlugin(pluginName)

type (
	HealthChecker struct {
		Next   plugin.Handler
		filter *HealthCheckFilter
	}
)

func (hc HealthChecker) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	qtype := r.Question[0].Qtype
	if !isSupportedType(qtype) {
		log.Debugf("unsupported type %s, nothing to do", dns.Type(qtype))
		return plugin.NextOrFailure(pluginName, hc.Next, ctx, w, r)
	}

	rw := NewResponseWriter(w, hc.filter)
	return plugin.NextOrFailure(pluginName, hc.Next, ctx, rw, r)
}

func (hc HealthChecker) Name() string { return pluginName }
