// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	discoveryV1beta1 "k8s.io/api/discovery/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Kubernetes implements a plugin that connects to a Kubernetes cluster.
type Kubernetes struct {
	Next             plugin.Handler
	Zones            []string
	Upstream         *upstream.Upstream
	APIServerList    []string
	APICertAuth      string
	APIClientCert    string
	APIClientKey     string
	ClientConfig     clientcmd.ClientConfig
	APIConn          dnsController
	Namespaces       map[string]struct{}
	podMode          string
	endpointNameMode bool
	Fall             fall.F
	ttl              uint32
	opts             dnsControlOpts
	primaryZoneIndex int
	localIPs         []net.IP
	autoPathSearch   []string // Local search path from /etc/resolv.conf. Needed for autopath.
}

// New returns a initialized Kubernetes. It default interfaceAddrFunc to return 127.0.0.1. All other
// values default to their zero value, primaryZoneIndex will thus point to the first zone.
func New(zones []string) *Kubernetes {
	k := new(Kubernetes)
	k.Zones = zones
	k.Namespaces = make(map[string]struct{})
	k.podMode = podModeDisabled
	k.ttl = defaultTTL

	return k
}

const (
	// podModeDisabled is the default value where pod requests are ignored
	podModeDisabled = "disabled"
	// podModeVerified is where Pod requests are answered only if they exist
	podModeVerified = "verified"
	// podModeInsecure is where pod requests are answered without verifying they exist
	podModeInsecure = "insecure"
	// DNSSchemaVersion is the schema version: https://github.com/kubernetes/dns/blob/master/docs/specification.md
	DNSSchemaVersion = "1.1.0"
	// Svc is the DNS schema for kubernetes services
	Svc = "svc"
	// Pod is the DNS schema for kubernetes pods
	Pod = "pod"
	// defaultTTL to apply to all answers.
	defaultTTL = 5
)

var (
	errNoItems        = errors.New("no items found")
	errNsNotExposed   = errors.New("namespace is not exposed")
	errInvalidRequest = errors.New("invalid query name")
)

// Services implements the ServiceBackend interface.
func (k *Kubernetes) Services(ctx context.Context, state request.Request, exact bool, opt plugin.Options) (svcs []msg.Service, err error) {
	// We're looking again at types, which we've already done in ServeDNS, but there are some types k8s just can't answer.
	switch state.QType() {

	case dns.TypeTXT:
		// 1 label + zone, label must be "dns-version".
		t, _ := dnsutil.TrimZone(state.Name(), state.Zone)

		segs := dns.SplitDomainName(t)
		if len(segs) != 1 {
			return nil, nil
		}
		if segs[0] != "dns-version" {
			return nil, nil
		}
		svc := msg.Service{Text: DNSSchemaVersion, TTL: 28800, Key: msg.Path(state.QName(), coredns)}
		return []msg.Service{svc}, nil

	case dns.TypeNS:
		// We can only get here if the qname equals the zone, see ServeDNS in handler.go.
		nss := k.nsAddrs(false, state.Zone)
		var svcs []msg.Service
		for _, ns := range nss {
			if ns.Header().Rrtype == dns.TypeA {
				svcs = append(svcs, msg.Service{Host: ns.(*dns.A).A.String(), Key: msg.Path(ns.Header().Name, coredns), TTL: k.ttl})
				continue
			}
			if ns.Header().Rrtype == dns.TypeAAAA {
				svcs = append(svcs, msg.Service{Host: ns.(*dns.AAAA).AAAA.String(), Key: msg.Path(ns.Header().Name, coredns), TTL: k.ttl})
			}
		}
		return svcs, nil
	}

	if isDefaultNS(state.Name(), state.Zone) {
		nss := k.nsAddrs(false, state.Zone)
		var svcs []msg.Service
		for _, ns := range nss {
			if ns.Header().Rrtype == dns.TypeA && state.QType() == dns.TypeA {
				svcs = append(svcs, msg.Service{Host: ns.(*dns.A).A.String(), Key: msg.Path(state.QName(), coredns), TTL: k.ttl})
				continue
			}
			if ns.Header().Rrtype == dns.TypeAAAA && state.QType() == dns.TypeAAAA {
				svcs = append(svcs, msg.Service{Host: ns.(*dns.AAAA).AAAA.String(), Key: msg.Path(state.QName(), coredns), TTL: k.ttl})
			}
		}
		return svcs, nil
	}

	s, e := k.Records(ctx, state, false)

	// SRV for external services is not yet implemented, so remove those records.

	if state.QType() != dns.TypeSRV {
		return s, e
	}

	internal := []msg.Service{}
	for _, svc := range s {
		if t, _ := svc.HostType(); t != dns.TypeCNAME {
			internal = append(internal, svc)
		}
	}

	return internal, e
}

// primaryZone will return the first non-reverse zone being handled by this plugin
func (k *Kubernetes) primaryZone() string { return k.Zones[k.primaryZoneIndex] }

// Lookup implements the ServiceBackend interface.
func (k *Kubernetes) Lookup(ctx context.Context, state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return k.Upstream.Lookup(ctx, state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (k *Kubernetes) IsNameError(err error) bool {
	return err == errNoItems || err == errNsNotExposed || err == errInvalidRequest
}

func (k *Kubernetes) getClientConfig() (*rest.Config, error) {
	if k.ClientConfig != nil {
		return k.ClientConfig.ClientConfig()
	}
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	overrides := &clientcmd.ConfigOverrides{}
	clusterinfo := clientcmdapi.Cluster{}
	authinfo := clientcmdapi.AuthInfo{}

	// Connect to API from in cluster
	if len(k.APIServerList) == 0 {
		cc, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		cc.ContentType = "application/vnd.kubernetes.protobuf"
		return cc, err
	}

	// Connect to API from out of cluster
	// Only the first one is used. We will deprecate multiple endpoints later.
	clusterinfo.Server = k.APIServerList[0]

	if len(k.APICertAuth) > 0 {
		clusterinfo.CertificateAuthority = k.APICertAuth
	}
	if len(k.APIClientCert) > 0 {
		authinfo.ClientCertificate = k.APIClientCert
	}
	if len(k.APIClientKey) > 0 {
		authinfo.ClientKey = k.APIClientKey
	}

	overrides.ClusterInfo = clusterinfo
	overrides.AuthInfo = authinfo
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	cc, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	cc.ContentType = "application/vnd.kubernetes.protobuf"
	return cc, err

}

// InitKubeCache initializes a new Kubernetes cache.
func (k *Kubernetes) InitKubeCache(ctx context.Context) (onStart func() error, onShut func() error, err error) {
	config, err := k.getClientConfig()
	if err != nil {
		return nil, nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kubernetes notification controller: %q", err)
	}

	if k.opts.labelSelector != nil {
		var selector labels.Selector
		selector, err = meta.LabelSelectorAsSelector(k.opts.labelSelector)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to create Selector for LabelSelector '%s': %q", k.opts.labelSelector, err)
		}
		k.opts.selector = selector
	}

	if k.opts.namespaceLabelSelector != nil {
		var selector labels.Selector
		selector, err = meta.LabelSelectorAsSelector(k.opts.namespaceLabelSelector)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to create Selector for LabelSelector '%s': %q", k.opts.namespaceLabelSelector, err)
		}
		k.opts.namespaceSelector = selector
	}

	k.opts.initPodCache = k.podMode == podModeVerified

	k.opts.zones = k.Zones
	k.opts.endpointNameMode = k.endpointNameMode

	k.APIConn = newdnsController(ctx, kubeClient, k.opts)

	initEndpointWatch := k.opts.initEndpointsCache

	onStart = func() error {
		go func() {
			if initEndpointWatch {
				// Revert to watching Endpoints for incompatible K8s.
				// This can be removed when all supported k8s versions support endpointslices.
				ok, v := k.endpointSliceSupported(kubeClient)
				if !ok {
					k.APIConn.(*dnsControl).WatchEndpoints(ctx)
				}
				// Revert to EndpointSlice v1beta1 if v1 is not supported
				if ok && v == discoveryV1beta1.SchemeGroupVersion.String() {
					k.APIConn.(*dnsControl).WatchEndpointSliceV1beta1(ctx)
				}
			}
			k.APIConn.Run()
		}()

		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if k.APIConn.HasSynced() {
					return nil
				}
			case <-timeout:
				log.Warning("starting server with unsynced Kubernetes API")
				return nil
			}
		}
	}

	onShut = func() error {
		return k.APIConn.Stop()
	}

	return onStart, onShut, err
}

// endpointSliceSupported will determine which endpoint object type to watch (endpointslices or endpoints)
// based on the supportability of endpointslices in the API and server version. It will return true when endpointslices
// should be watched, and false when endpoints should be watched.
// If the API supports discovery, and the server versions >= 1.19, true is returned.
// Also returned is the discovery version supported: "v1" if v1 is supported, and v1beta1 if v1beta1 is supported and
// v1 is not supported.
// This function should be removed, when all supported versions of k8s support v1.
func (k *Kubernetes) endpointSliceSupported(kubeClient *kubernetes.Clientset) (bool, string) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sv, err := kubeClient.ServerVersion()
			if err != nil {
				continue
			}

			// Disable use of endpoint slices for k8s versions 1.18 and earlier. The Endpointslices API was enabled
			// by default in 1.17 but Service -> Pod proxy continued to use Endpoints by default until 1.19.
			// DNS results should be built from the same source data that the proxy uses.  This decision assumes
			// k8s EndpointSliceProxying featuregate is at the default (i.e. only enabled for k8s >= 1.19).
			major, _ := strconv.Atoi(sv.Major)
			minor, _ := strconv.Atoi(strings.TrimRight(sv.Minor, "+"))
			if major <= 1 && minor <= 18 {
				log.Info("Watching Endpoints instead of EndpointSlices in k8s versions < 1.19")
				return false, ""
			}

			// Enable use of endpoint slices if the API supports the discovery api
			_, err = kubeClient.Discovery().ServerResourcesForGroupVersion(discovery.SchemeGroupVersion.String())
			if err == nil {
				return true, discovery.SchemeGroupVersion.String()
			} else if !kerrors.IsNotFound(err) {
				continue
			}

			_, err = kubeClient.Discovery().ServerResourcesForGroupVersion(discoveryV1beta1.SchemeGroupVersion.String())
			if err == nil {
				return true, discoveryV1beta1.SchemeGroupVersion.String()
			} else if !kerrors.IsNotFound(err) {
				continue
			}

			// Disable use of endpoint slices in case that it is disabled in k8s versions 1.19 and newer.
			log.Info("Endpointslices API disabled. Watching Endpoints instead.")
			return false, ""
		}
	}
}

// Records looks up services in kubernetes.
func (k *Kubernetes) Records(ctx context.Context, state request.Request, exact bool) ([]msg.Service, error) {
	r, e := parseRequest(state.Name(), state.Zone)
	if e != nil {
		return nil, e
	}
	if r.podOrSvc == "" {
		return nil, nil
	}

	if dnsutil.IsReverse(state.Name()) > 0 {
		return nil, errNoItems
	}

	if !wildcard(r.namespace) && !k.namespaceExposed(r.namespace) {
		return nil, errNsNotExposed
	}

	if r.podOrSvc == Pod {
		pods, err := k.findPods(r, state.Zone)
		return pods, err
	}

	services, err := k.findServices(r, state.Zone)
	return services, err
}

func endpointHostname(addr object.EndpointAddress, endpointNameMode bool) string {
	if addr.Hostname != "" {
		return addr.Hostname
	}
	if endpointNameMode && addr.TargetRefName != "" {
		return addr.TargetRefName
	}
	if strings.Contains(addr.IP, ".") {
		return strings.Replace(addr.IP, ".", "-", -1)
	}
	if strings.Contains(addr.IP, ":") {
		return strings.Replace(addr.IP, ":", "-", -1)
	}
	return ""
}

func (k *Kubernetes) findPods(r recordRequest, zone string) (pods []msg.Service, err error) {
	if k.podMode == podModeDisabled {
		return nil, errNoItems
	}

	namespace := r.namespace
	if !wildcard(namespace) && !k.namespaceExposed(namespace) {
		return nil, errNoItems
	}

	podname := r.service

	// handle empty pod name
	if podname == "" {
		if k.namespaceExposed(namespace) || wildcard(namespace) {
			// NODATA
			return nil, nil
		}
		// NXDOMAIN
		return nil, errNoItems
	}

	zonePath := msg.Path(zone, coredns)
	ip := ""
	if strings.Count(podname, "-") == 3 && !strings.Contains(podname, "--") {
		ip = strings.Replace(podname, "-", ".", -1)
	} else {
		ip = strings.Replace(podname, "-", ":", -1)
	}

	if k.podMode == podModeInsecure {
		if !wildcard(namespace) && !k.namespaceExposed(namespace) { // no wildcard, but namespace does not exist
			return nil, errNoItems
		}

		// If ip does not parse as an IP address, we return an error, otherwise we assume a CNAME and will try to resolve it in backend_lookup.go
		if net.ParseIP(ip) == nil {
			return nil, errNoItems
		}

		return []msg.Service{{Key: strings.Join([]string{zonePath, Pod, namespace, podname}, "/"), Host: ip, TTL: k.ttl}}, err
	}

	// PodModeVerified
	err = errNoItems
	if wildcard(podname) && !wildcard(namespace) {
		// If namespace exists, err should be nil, so that we return NODATA instead of NXDOMAIN
		if k.namespaceExposed(namespace) {
			err = nil
		}
	}

	for _, p := range k.APIConn.PodIndex(ip) {
		// If namespace has a wildcard, filter results against Corefile namespace list.
		if wildcard(namespace) && !k.namespaceExposed(p.Namespace) {
			continue
		}

		// check for matching ip and namespace
		if ip == p.PodIP && match(namespace, p.Namespace) {
			s := msg.Service{Key: strings.Join([]string{zonePath, Pod, namespace, podname}, "/"), Host: ip, TTL: k.ttl}
			pods = append(pods, s)

			err = nil
		}
	}
	return pods, err
}

// findServices returns the services matching r from the cache.
func (k *Kubernetes) findServices(r recordRequest, zone string) (services []msg.Service, err error) {
	if !wildcard(r.namespace) && !k.namespaceExposed(r.namespace) {
		return nil, errNoItems
	}

	// handle empty service name
	if r.service == "" {
		if k.namespaceExposed(r.namespace) || wildcard(r.namespace) {
			// NODATA
			return nil, nil
		}
		// NXDOMAIN
		return nil, errNoItems
	}

	err = errNoItems
	if wildcard(r.service) && !wildcard(r.namespace) {
		// If namespace exists, err should be nil, so that we return NODATA instead of NXDOMAIN
		if k.namespaceExposed(r.namespace) {
			err = nil
		}
	}

	var (
		endpointsListFunc func() []*object.Endpoints
		endpointsList     []*object.Endpoints
		serviceList       []*object.Service
	)

	if wildcard(r.service) || wildcard(r.namespace) {
		serviceList = k.APIConn.ServiceList()
		endpointsListFunc = func() []*object.Endpoints { return k.APIConn.EndpointsList() }
	} else {
		idx := object.ServiceKey(r.service, r.namespace)
		serviceList = k.APIConn.SvcIndex(idx)
		endpointsListFunc = func() []*object.Endpoints { return k.APIConn.EpIndex(idx) }
	}

	zonePath := msg.Path(zone, coredns)
	for _, svc := range serviceList {
		if !(match(r.namespace, svc.Namespace) && match(r.service, svc.Name)) {
			continue
		}

		// If request namespace is a wildcard, filter results against Corefile namespace list.
		// (Namespaces without a wildcard were filtered before the call to this function.)
		if wildcard(r.namespace) && !k.namespaceExposed(svc.Namespace) {
			continue
		}

		// If "ignore empty_service" option is set and no endpoints exist, return NXDOMAIN unless
		// it's a headless or externalName service (covered below).
		if k.opts.ignoreEmptyService && svc.Type != api.ServiceTypeExternalName && !svc.Headless() { // serve NXDOMAIN if no endpoint is able to answer
			podsCount := 0
			for _, ep := range endpointsListFunc() {
				for _, eps := range ep.Subsets {
					podsCount = podsCount + len(eps.Addresses)
				}
			}

			if podsCount == 0 {
				continue
			}
		}

		// External service
		if svc.Type == api.ServiceTypeExternalName {
			s := msg.Service{Key: strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/"), Host: svc.ExternalName, TTL: k.ttl}
			if t, _ := s.HostType(); t == dns.TypeCNAME {
				s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/")
				services = append(services, s)

				err = nil
			}
			continue
		}

		// Endpoint query or headless service
		if svc.Headless() || r.endpoint != "" {
			if endpointsList == nil {
				endpointsList = endpointsListFunc()
			}

			for _, ep := range endpointsList {
				if object.EndpointsKey(svc.Name, svc.Namespace) != ep.Index {
					continue
				}

				for _, eps := range ep.Subsets {
					for _, addr := range eps.Addresses {

						// See comments in parse.go parseRequest about the endpoint handling.
						if r.endpoint != "" {
							if !match(r.endpoint, endpointHostname(addr, k.endpointNameMode)) {
								continue
							}
						}

						for _, p := range eps.Ports {
							if !(match(r.port, p.Name) && match(r.protocol, string(p.Protocol))) {
								continue
							}
							s := msg.Service{Host: addr.IP, Port: int(p.Port), TTL: k.ttl}
							s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name, endpointHostname(addr, k.endpointNameMode)}, "/")

							err = nil

							services = append(services, s)
						}
					}
				}
			}
			continue
		}

		// ClusterIP service
		for _, p := range svc.Ports {
			if !(match(r.port, p.Name) && match(r.protocol, string(p.Protocol))) {
				continue
			}

			err = nil

			for _, ip := range svc.ClusterIPs {
				s := msg.Service{Host: ip, Port: int(p.Port), TTL: k.ttl}
				s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/")
				services = append(services, s)
			}
		}
	}
	return services, err
}

// Serial return the SOA serial.
func (k *Kubernetes) Serial(state request.Request) uint32 { return uint32(k.APIConn.Modified()) }

// MinTTL returns the minimal TTL.
func (k *Kubernetes) MinTTL(state request.Request) uint32 { return k.ttl }

// match checks if a and b are equal taking wildcards into account.
func match(a, b string) bool {
	if wildcard(a) {
		return true
	}
	if wildcard(b) {
		return true
	}
	return strings.EqualFold(a, b)
}

// wildcard checks whether s contains a wildcard value defined as "*" or "any".
func wildcard(s string) bool {
	return s == "*" || s == "any"
}

const coredns = "c" // used as a fake key prefix in msg.Service
