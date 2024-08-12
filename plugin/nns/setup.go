package nns

import (
	"context"
	"fmt"
	"net/url"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/nns/contract"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const pluginName = "nns"

func init() {
	plugin.Register(pluginName, setup)
}

func setup(c *caddy.Controller) error {
	URL, err := url.Parse(c.Key)
	if err != nil {
		return plugin.Error(pluginName, c.Err(err.Error()))
	}

	contractParams, err := parseContractParams(c)
	if err != nil {
		return err
	}

	ctx := context.TODO()

	contracts := make([]*contract.Contract, len(contractParams))
	for i, prm := range contractParams {
		contracts[i], err = contract.NewContract(ctx, prm)
		if err != nil {
			return plugin.Error(pluginName, c.Err(err.Error()))
		}
	}

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		nns := &NNS{
			Next:      next,
			Contracts: contracts,
			Log:       clog.NewWithPlugin(pluginName),
		}
		nns.setDNSDomain(URL.Hostname())

		return *nns
	})

	return nil
}

func parseContractParams(c *caddy.Controller) ([]*contract.Params, error) {
	var result []*contract.Params
	for c.Next() {
		prm, err := parseContractParam(c.RemainingArgs())
		if err != nil {
			return nil, err
		}
		result = append(result, prm)
	}

	return result, nil
}

func parseContractParam(args []string) (*contract.Params, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, plugin.Error(pluginName, fmt.Errorf("support the following args template: 'NEO_CHAIN_ENDPOINT CONTRACT_ADDRESS [NNS_DOMAIN]'"))
	}

	prm := &contract.Params{Endpoint: args[0]}
	if URL, err := url.Parse(prm.Endpoint); err != nil {
		return nil, plugin.Error(pluginName, fmt.Errorf("couldn't parse endpoint: %w", err))
	} else if URL.Scheme == "" || URL.Port() == "" {
		return nil, plugin.Error(pluginName, fmt.Errorf("invalid endpoint: %s", prm.Endpoint))
	}

	hexStr := args[1]
	if hexStr != "-" {
		hash, err := util.Uint160DecodeStringLE(hexStr)
		if err != nil {
			return nil, plugin.Error(pluginName, fmt.Errorf("invalid nns contract address"))
		}
		prm.ContractHash = hash
	}

	if len(args) == 3 {
		prm.Domain = args[2]
	}

	return prm, nil
}
