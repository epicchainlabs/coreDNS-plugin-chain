package geodns

import (
	"testing"

	"github.com/coredns/caddy"
)

func TestSetup(t *testing.T) {
	for _, tc := range []struct {
		args  string
		valid bool
	}{
		{args: "", valid: false},
		{args: "path1", valid: false},
		{args: "path1 path2", valid: false},
		{args: "path1 arg2 arg3", valid: false},
		{args: "testdata/GeoIP2-City-Test.mmdb -1", valid: false},
		{args: "testdata/", valid: true},
		{args: "testdata 3", valid: true},
	} {
		c := caddy.NewTestController("dns", "geodns "+tc.args)
		err := setup(c)
		if tc.valid && err != nil {
			t.Fatalf("Expected no errors, but got: %v", err)
		} else if !tc.valid && err == nil {
			t.Fatalf("Expected errors, but got: %v", err)
		}
	}
}
