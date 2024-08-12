package geodns

import (
	"fmt"
	"strconv"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

const pluginName = "geodns"

func init() {
	plugin.Register(pluginName, setup)
}

func setup(c *caddy.Controller) error {
	geoDNS, err := geoDNSParse(c)
	if err != nil {
		return plugin.Error(pluginName, err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		geoDNS.Next = next
		return geoDNS
	})

	return nil
}

func geoDNSParse(c *caddy.Controller) (*GeoDNS, error) {
	c.Next()
	args := c.RemainingArgs()
	if len(args) < 1 || len(args) > 2 {
		return nil, plugin.Error(pluginName, fmt.Errorf("support the following args template: 'GEOIP_DATABASE [MAX_RECORDS]'"))
	}
	dbPath := args[0]
	maxRecords := 1
	if len(args) == 2 {
		max, err := strconv.Atoi(args[1])
		if err != nil || max < 1 {
			return nil, plugin.Error(pluginName, fmt.Errorf("invalid max records arg: %s", args[1]))
		}
		maxRecords = max
	}

	geoDNS, err := newGeoDNS(dbPath, maxRecords)
	if err != nil {
		return geoDNS, c.Err(err.Error())
	}
	return geoDNS, nil
}
