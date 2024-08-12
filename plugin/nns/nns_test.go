package nns

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/nns/contract"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestGetNetmapHash(t *testing.T) {
	ctx := context.Background()
	container := createDockerContainer(ctx, t, testImage)
	defer container.Terminate(ctx)

	prm := &contract.Params{
		Endpoint: "http://localhost:30333",
	}
	nnsContract, err := contract.NewContract(ctx, prm)
	require.NoError(t, err)

	nns := NNS{
		Next:      test.NextHandler(dns.RcodeSuccess, nil),
		Contracts: []*contract.Contract{nnsContract},
	}

	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("netmap.neofs"), dns.TypeTXT)
	req.Question[0].Qclass = dns.ClassCHAOS

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	status, err := nns.ServeDNS(context.TODO(), rec, req)
	require.NoError(t, err)
	require.Equal(t, dns.RcodeSuccess, status)

	res := rec.Msg.Answer[0].(*dns.TXT).Txt[0]
	require.Equal(t, "0605a9623cb07b638fc6fe243bb7dc8bc50d30cd", res)

	t.Run("prepare names", func(t *testing.T) {
		for _, tc := range []struct {
			dnsDomain string
			nnsDomain string
			request   string
			expected  string
		}{
			{
				dnsDomain: ".",
				nnsDomain: "",
				request:   "test.neofs",
				expected:  "test.neofs",
			},
			{
				dnsDomain: ".",
				nnsDomain: "",
				request:   "test.neofs.",
				expected:  "test.neofs",
			},
			{
				dnsDomain: ".",
				nnsDomain: "container.",
				request:   "test.neofs",
				expected:  "test.neofs.container",
			},
			{
				dnsDomain: ".",
				nnsDomain: ".container",
				request:   "test.neofs.",
				expected:  "test.neofs.container",
			},
			{
				dnsDomain: "containers.testnet.fs.neo.org.",
				nnsDomain: "container",
				request:   "containers.testnet.fs.neo.org",
				expected:  "container",
			},
			{
				dnsDomain: ".containers.testnet.fs.neo.org",
				nnsDomain: "container",
				request:   "containers.testnet.fs.neo.org.",
				expected:  "container",
			},
			{
				dnsDomain: "containers.testnet.fs.neo.org.",
				nnsDomain: "container",
				request:   "nicename.containers.testnet.fs.neo.org",
				expected:  "nicename.container",
			},
		} {
			t.Run("", func(t *testing.T) {
				nnsPlugin := &NNS{}
				nnsPlugin.setDNSDomain(tc.dnsDomain)

				contractPrm := &contract.Params{
					Endpoint: prm.Endpoint,
					Domain:   tc.nnsDomain,
				}
				contractNNS, err := contract.NewContract(ctx, contractPrm)
				require.NoError(t, err)

				result := contractNNS.PrepareName(tc.request, nnsPlugin.dnsDomain)
				require.Equal(t, tc.expected, result)
			})
		}
	})
}
