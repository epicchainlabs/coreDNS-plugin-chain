package healthchecker

import (
	"testing"

	"github.com/coredns/caddy"
)

func TestSetup(t *testing.T) {
	for _, tc := range []struct {
		args  string
		valid bool
	}{
		// unknown
		{args: "foo", valid: false},
		{args: "", valid: false},
		// not enough args
		{args: "http", valid: false},
		{args: "http 1000", valid: false},
		{args: "http 1000 1s", valid: false},
		// http method params check, 100, 3s, fs.neo.org. are valid cache size, check interval and origin
		{args: "http 100 1s fs.neo.org. @", valid: true},
		{args: `http 100 1s fs.neo.org. {
				port asdf
			}`, valid: false},
		{args: `http 100 1s fs.neo.org. {
				port 0
			}`, valid: false},
		{args: `http 100 1s fs.neo.org. {
				port -1
			}`, valid: false},
		{args: `http 100 1s fs.neo.org. {
				port 80
			}`, valid: true},
		{args: `http 100 1s fs.neo.org. {
				port 80 80
			}`, valid: false},
		{args: `http 100 1s fs.neo.org. {
				port 80
				timeout 3s
			}`, valid: true},
		{args: `http 100 1s fs.neo.org. {
				port 80
				timeout 3s 3s
			}`, valid: false},
		{args: `http 100 1s fs.neo.org. {
				port 80
				timeout 0
			}`, valid: false},
		{args: `http 100 1s fs.neo.org. {
				port 80
				timeout seconds
			}`, valid: false},
		// icmp method params check
		{args: "icmp 100 1s fs.neo.org. @", valid: true},
		{args: `icmp 100 1s fs.neo.org. {
				timeout 3s
			}`, valid: true},
		{args: `icmp 100 1s fs.neo.org. {
				timeout 3s 3s
			}`, valid: false},
		{args: `icmp 100 1s fs.neo.org. {
				timeout 0
			}`, valid: false},
		{args: `icmp 100 1s fs.neo.org. {
				timeout seconds
			}`, valid: false},
		{args: `icmp 100 1s fs.neo.org. {
				privileged
			}`, valid: true},
		{args: `icmp 100 1s fs.neo.org. {
				privileged
				timeout 3s
			}`, valid: true},
		{args: `icmp 100 1s fs.neo.org. {
				privileged true
			}`, valid: false},
		// cache size
		{args: "http -1 1s fs.neo.org.", valid: false},
		{args: "http 100a 1s fs.neo.org.", valid: false},
		{args: "http 0 1s fs.neo.org.", valid: false},
		{args: "icmp -1 1s fs.neo.org.", valid: false},
		{args: "icmp 100a 1s fs.neo.org.", valid: false},
		{args: "icmp 0 1s fs.neo.org.", valid: false},
		// check interval, test with a valid value is above
		{args: "http 100 0h fs.neo.org.", valid: false},
		{args: "http 100 100 fs.neo.org.", valid: false},
		{args: "http 100 3000a fs.neo.org.", valid: false},
		{args: "http 100 -1m fs.neo.org.", valid: false},
		{args: "icmp 100 0h fs.neo.org.", valid: false},
		{args: "icmp 100 100 fs.neo.org.", valid: false},
		{args: "icmp 100 3000a fs.neo.org.", valid: false},
		{args: "icmp 100 -1m fs.neo.org.", valid: false},
		// origin, test with a valid value is above
		{args: "http 100 3000 fs.neo.org", valid: false},
		{args: "icmp 100 3000 fs.neo.org", valid: false},
		// names
		{args: "http 100 3m fs.neo.org. @ kimchi", valid: true},
		{args: "http 100 3m fs.neo.org. ^cdn\\.fs\\.\\neo\\.org", valid: true},
		{args: "http 100 3m \\uFFFD", valid: false},
		{args: "icmp 100 3m fs.neo.org. @ kimchi", valid: true},
		{args: "icmp 100 3m fs.neo.org. ^cdn\\.fs\\.\\neo\\.org", valid: true},
		{args: "icmp 100 3m \\uFFFD", valid: false},
	} {
		c := caddy.NewTestController("dns", "healthchecker "+tc.args)
		err := setup(c)
		if tc.valid && err != nil {
			t.Fatalf("Expected no errors, but got: %v, test case: '%s'", err, tc.args)
		} else if !tc.valid && err == nil {
			t.Fatalf("Expected errors, but got: %v, test case: '%s'", err, tc.args)
		}
	}
}
