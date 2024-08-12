package healthchecker

import (
	"fmt"
	"regexp"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	"go.uber.org/atomic"
)

type (
	HealthCheckFilter struct {
		cache    *lru.Cache
		checker  Checker
		interval time.Duration
		names    map[string]struct{}
		filters  []Filter
	}

	entry struct {
		endpoint string
		healthy  *atomic.Bool
		quit     chan struct{}
	}

	Checker interface {
		Check(record string) bool
	}

	Filter interface {
		Match(string) bool
	}

	RegexpFilter struct {
		expr *regexp.Regexp
	}

	SimpleMatchFilter string
)

func (f SimpleMatchFilter) Match(rec string) bool {
	return string(f) == rec
}

func NewRegexpFilter(pattern string) (*RegexpFilter, error) {
	expr, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexpFilter{expr: expr}, nil
}

func (f *RegexpFilter) Match(rec string) bool {
	return f.expr.MatchString(rec)
}

func NewHealthCheckFilter(checker Checker, size int, interval time.Duration, filters []Filter) (*HealthCheckFilter, error) {
	if len(filters) == 0 {
		return nil, fmt.Errorf("filters must not be empty")
	}

	cache, err := lru.NewWithEvict(size, func(key interface{}, value interface{}) {
		if e, ok := value.(*entry); ok {
			close(e.quit)
		}
	})
	if err != nil {
		return nil, err
	}
	return &HealthCheckFilter{
		cache:    cache,
		checker:  checker,
		interval: interval,
		filters:  filters,
	}, nil
}

func (p *HealthCheckFilter) FilterRecords(records []dns.RR) []dns.RR {
	result := make([]dns.RR, 0, len(records))

	for _, r := range records {
		if matchFilters(p.filters, r.Header().Name) {
			endpoint, err := getEndpoint(r)
			if err != nil {
				log.Warningf("record will be ignored: %s", err.Error())
				continue
			}
			e := p.get(endpoint)
			if e != nil {
				if e.healthy.Load() {
					result = append(result, r)
				}
				continue
			}
			p.put(endpoint)
			log.Debugf("record '%s' will be cached", r.String())
		}
		result = append(result, r)
	}

	return result
}

func getEndpoint(record dns.RR) (string, error) {
	var endpoint string
	if aRec, ok := record.(*dns.A); ok {
		endpoint = aRec.A.String()
	} else if aaaaRec, ok := record.(*dns.AAAA); ok {
		endpoint = aaaaRec.AAAA.String()
	} else {
		// types should have been filtered before, it's something odd if we are here
		return "", fmt.Errorf("not supported record type: %s", record.String())
	}

	return endpoint, nil
}

func matchFilters(filters []Filter, record string) bool {
	for _, filter := range filters {
		if filter.Match(record) {
			return true
		}
	}

	return false
}

func (p *HealthCheckFilter) put(endpoint string) {
	health := p.checker.Check(endpoint)
	quit := make(chan struct{})
	record := &entry{
		endpoint: endpoint,
		healthy:  atomic.NewBool(health),
		quit:     quit,
	}
	p.cache.Add(endpoint, record)

	ticker := time.NewTicker(p.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				e, ok := p.cache.Peek(endpoint)
				if !ok {
					return
				}
				val, ok := e.(*entry)
				if !ok {
					return
				}
				val.healthy.Store(p.checker.Check(endpoint))
				ticker.Reset(p.interval)
			}
		}
	}()
}

func (p *HealthCheckFilter) get(key string) *entry {
	val, ok := p.cache.Get(key)
	if !ok {
		return nil
	}

	var result *entry
	result, ok = val.(*entry)
	if !ok {
		return nil
	}

	return result
}
