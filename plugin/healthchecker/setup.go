package healthchecker

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/healthchecker/checkers"
)

const (
	httpChecker = "http"
	icmpChecker = "icmp"
)

func init() {
	plugin.Register(pluginName, setup)
}

func setup(c *caddy.Controller) error {
	c.Next()
	filter, err := filterParamsParse(c)
	if err != nil {
		return err
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return HealthChecker{
			Next:   next,
			filter: filter,
		}
	})

	return nil
}

func filterParamsParse(c *caddy.Controller) (*HealthCheckFilter, error) {
	var checker Checker
	var err error
	args := c.RemainingArgs()
	if len(args) < 4 {
		return nil, plugin.Error(pluginName,
			fmt.Errorf("the following format is supported: HEALTHCHECK_METHOD CACHE_SIZE "+
				"HEALTHCHECK_INTERVAL_IN_MS REGEXP_FILTER [ADDITIONAL_REGEXP_FILTERS... ]"))
	}

	checkerType := args[0]
	switch checkerType {
	case httpChecker:
		var prm *checkers.HTTPCheckerParams
		if prm, err = checkers.ParseHTTPParams(c); err == nil {
			checker, err = checkers.NewHttpChecker(log, prm)
		}
	case icmpChecker:
		var prm *checkers.ICMPCheckerParams
		if prm, err = checkers.ParseICMPParams(c); err == nil {
			checker, err = checkers.NewICMPChecker(log, prm)
		}
	default:
		return nil, plugin.Error(pluginName, fmt.Errorf("unsupported checker type: '%s'", checkerType))
	}
	if err != nil {
		return nil, plugin.Error(pluginName, err)
	}

	URL, err := url.Parse(c.Key)
	if err != nil {
		return nil, err
	}
	origin := URL.Hostname()

	//parsing cache size
	size, err := strconv.Atoi(args[1])
	if err != nil || size <= 0 {
		return nil, plugin.Error(pluginName, fmt.Errorf("invalid cache size: %s", args[1]))
	}

	// parsing check interval
	interval, err := time.ParseDuration(args[2])
	if err != nil || interval <= 0 {
		return nil, plugin.Error(pluginName, fmt.Errorf("invalid endpoint check interval: %s", args[2]))
	}

	// parsing filters
	var filter Filter
	filters := make([]Filter, 0, len(args[3:]))
	for _, rawFilter := range args[3:] {
		if rawFilter == "@" {
			filter = SimpleMatchFilter(origin)
		} else {
			filter, err = NewRegexpFilter(rawFilter)
			if err != nil {
				return nil, plugin.Error(pluginName, fmt.Errorf("invalid regexp filter: %s", rawFilter))
			}
		}
		filters = append(filters, filter)
	}

	healthCheckFilter, err := NewHealthCheckFilter(checker, size, interval, filters)
	if err != nil {
		return nil, plugin.Error(pluginName, fmt.Errorf("couldn't create healthcheck filter: %w", err))
	}

	return healthCheckFilter, nil
}
