package geodns

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
)

var log = clog.NewWithPlugin(pluginName)

type GeoDNS struct {
	Next   plugin.Handler
	filter *filter
}

type filter struct {
	db         *db
	maxRecords int
}

func newGeoDNS(dbPath string, maxRecords int) (*GeoDNS, error) {
	entries, err := os.ReadDir(dbPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't read dir with dbs: %w", err)
	}

	db := &db{readers: make(map[int]*geoip2.Reader)}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".mmdb") {
			continue
		}
		r, err := geoip2.Open(filepath.Join(dbPath, entry.Name()))
		if err != nil {
			fmt.Printf("failed to open database file: %s, error: %v\n", entry.Name(), err)
			continue
		}

		dbType, err := getDBType(r)
		if err != nil {
			fmt.Printf("failed to get database type: %s, error: %v\n", entry.Name(), err)
			continue
		}
		db.AddReader(dbType, r)
		count++
		fmt.Printf("%s geoip db was added, type: %s\n", entry.Name(), typeToString(dbType))
	}
	fmt.Printf("Configured %d dbs. Note: when several db the same type add the last one will be used\n", count)

	return &GeoDNS{
		filter: &filter{
			db:         db,
			maxRecords: maxRecords,
		},
	}, nil
}

// ServeDNS implements the plugin.Handler interface.
func (g GeoDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	qtype := r.Question[0].Qtype

	if !isSupportedType(qtype) {
		log.Debugf("unsupported type %s, nothing to do", dns.Type(qtype))
		return plugin.NextOrFailure(pluginName, g.Next, ctx, w, r)
	}

	var realIP net.IP

	if addr, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		realIP = make(net.IP, len(addr.IP))
		copy(realIP, addr.IP)
	} else if addr, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		realIP = make(net.IP, len(addr.IP))
		copy(realIP, addr.IP)
	}

	var ip net.IP // EDNS CLIENT SUBNET or real IP
	if option := r.IsEdns0(); option != nil {
		for _, s := range option.Option {
			switch e := s.(type) {
			case *dns.EDNS0_SUBNET:
				log.Debug("Got edns-client-subnet", e.Address, e.Family, e.SourceNetmask, e.SourceScope)
				if e.Address != nil {
					ip = e.Address
				}
			}
		}
	}

	if len(ip) == 0 { // no edns client subnet
		ip = realIP
	}

	rw := NewResponseFilter(w, g.filter, ip)
	return plugin.NextOrFailure(pluginName, g.Next, ctx, rw, r)
}

// Name implements the Handler interface.
func (g GeoDNS) Name() string { return pluginName }
