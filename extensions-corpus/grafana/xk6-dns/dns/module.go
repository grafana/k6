// Package dns provides a k6 module for DNS resolution and lookup.
package dns

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
	"go.k6.io/k6-extension-api/common"
)

// ImportPath is the JavaScript import path for the DNS module.
const ImportPath = "k6/x/dns"

// RootModule is the module that creates a DNS module instance for each VU.
type RootModule struct{}

// ModuleInstance is a DNS module instance for one VU.
type ModuleInstance struct {
	vu        extensionapi.VU
	dnsClient *Client
	metrics   *moduleInstanceMetrics
}

var _ extensionapi.Module = (*RootModule)(nil)
var _ extensionapi.Instance = (*ModuleInstance)(nil)

// New creates a new DNS module.
func New() extensionapi.Module { return new(RootModule) }

// NewModuleInstance creates a DNS module instance for vu.
func (*RootModule) NewModuleInstance(vu extensionapi.VU) extensionapi.Instance {
	instanceMetrics, err := registerMetrics(vu)
	if err != nil {
		common.Throw(vu.Runtime(), fmt.Errorf("register DNS metrics: %w", err))
	}

	return &ModuleInstance{
		vu:        vu,
		dnsClient: NewDNSClient(vu),
		metrics:   instanceMetrics,
	}
}

// Exports returns the module exports available in JavaScript.
func (mi *ModuleInstance) Exports() extensionapi.Exports {
	return extensionapi.Exports{Named: map[string]any{
		"resolve": mi.Resolve,
		"lookup":  mi.Lookup,
	}}
}

// Resolve resolves a domain name using the supplied nameserver.
func (mi *ModuleInstance) Resolve(query, recordType, nameserverAddr sobek.Value) *sobek.Promise {
	promise, resolver := newPromise(mi.vu)
	resolve, reject := resolver.Resolve, resolver.Reject

	if nameserverAddr == nil {
		reject(errors.New("nameserver argument must be provided"))
		return promise
	}

	var queryStr string
	if err := mi.vu.Runtime().ExportTo(query, &queryStr); err != nil {
		reject(fmt.Errorf("query must be a string; got %v instead", query))
		return promise
	}

	var recordTypeStr string
	if err := mi.vu.Runtime().ExportTo(recordType, &recordTypeStr); err != nil {
		reject(fmt.Errorf("recordType must be a string; got %v instead", recordType))
		return promise
	}

	var nameserverAddrStr string
	if err := mi.vu.Runtime().ExportTo(nameserverAddr, &nameserverAddrStr); err != nil {
		reject(fmt.Errorf("nameserver must be a string; got %v instead", nameserverAddr))
		return promise
	}

	nameserver, err := parseNameserverAddr(nameserverAddrStr)
	if err != nil {
		reject(fmt.Errorf("parsing nameserver address failed: %w", err))
		return promise
	}

	go func() {
		startedAt := time.Now()
		results, resolveErr := mi.dnsClient.Resolve(mi.vu.Context(), queryStr, recordTypeStr, nameserver)
		mi.emitResolutionMetrics(mi.vu.Context(), time.Since(startedAt), queryStr, recordTypeStr, nameserver, resolveErr)
		if resolveErr != nil {
			reject(resolveErr)
			return
		}
		resolve(results)
	}()

	return promise
}

// Lookup resolves a domain name using the host resolver.
func (mi *ModuleInstance) Lookup(hostname sobek.Value) *sobek.Promise {
	promise, resolver := newPromise(mi.vu)
	resolve, reject := resolver.Resolve, resolver.Reject

	var hostnameStr string
	if err := mi.vu.Runtime().ExportTo(hostname, &hostnameStr); err != nil {
		reject(fmt.Errorf("hostname must be a string; got %T instead", hostname))
		return promise
	}

	go func() {
		startedAt := time.Now()
		ips, lookupErr := mi.dnsClient.Lookup(mi.vu.Context(), hostnameStr)
		mi.emitLookupMetrics(mi.vu.Context(), time.Since(startedAt), hostnameStr, lookupErr)
		if lookupErr != nil {
			reject(lookupErr)
			return
		}
		resolve(ips)
	}()

	return promise
}

func newPromise(vu extensionapi.VU) (*sobek.Promise, extensionapi.PromiseResolver) {
	promises, ok := vu.(extensionapi.Promises)
	if !ok {
		panic("extension API promise capability is unavailable")
	}
	return promises.NewPromise()
}

// registerMetrics registers DNS metrics for one module instance.
func registerMetrics(vu extensionapi.VU) (*moduleInstanceMetrics, error) {
	host, ok := vu.(extensionapi.Metrics)
	if !ok {
		return nil, extensionapi.ErrMetricsUnavailable
	}
	register := func(name string, kind extensionapi.MetricKind, unit extensionapi.MetricUnit) (extensionapi.Metric, error) {
		return host.RegisterMetric(extensionapi.MetricSpec{Name: name, Kind: kind, Unit: unit})
	}
	m := &moduleInstanceMetrics{host: host}
	var err error
	if m.DNSResolutions, err = register("dns_resolutions", extensionapi.MetricCounter, extensionapi.MetricUnitDefault); err != nil {
		return nil, err
	}
	if m.DNSResolutionDuration, err = register("dns_resolution_duration", extensionapi.MetricTrend, extensionapi.MetricUnitTime); err != nil {
		return nil, err
	}
	if m.DNSResolutionFailed, err = register("dns_resolution_failed", extensionapi.MetricRate, extensionapi.MetricUnitDefault); err != nil {
		return nil, err
	}
	if m.DNSLookups, err = register("dns_lookups", extensionapi.MetricCounter, extensionapi.MetricUnitDefault); err != nil {
		return nil, err
	}
	if m.DNSLookupDuration, err = register("dns_lookup_duration", extensionapi.MetricTrend, extensionapi.MetricUnitTime); err != nil {
		return nil, err
	}
	if m.DNSLookupFailed, err = register("dns_lookup_failed", extensionapi.MetricRate, extensionapi.MetricUnitDefault); err != nil {
		return nil, err
	}
	return m, nil
}

func (mi *ModuleInstance) emitResolutionMetrics(
	ctx context.Context, duration time.Duration, query, recordType string, nameserver Nameserver, resolutionErr error,
) {
	tags := mi.metrics.host.CurrentTags().With(map[string]string{
		"query": query, "recordType": recordType, "nameserver": nameserver.Addr(),
	})
	failed := 0.0
	if resolutionErr != nil {
		failed = 1
	}
	_ = mi.metrics.host.Emit(ctx, []extensionapi.Sample{
		{Metric: mi.metrics.DNSResolutions, Value: 1, Tags: tags},
		{Metric: mi.metrics.DNSResolutionDuration, Value: float64(duration.Milliseconds()), Tags: tags},
		{Metric: mi.metrics.DNSResolutionFailed, Value: failed, Tags: tags},
	})
}

func (mi *ModuleInstance) emitLookupMetrics(ctx context.Context, duration time.Duration, host string, lookupErr error) {
	tags := mi.metrics.host.CurrentTags().With(map[string]string{"host": host})
	failed := 0.0
	if lookupErr != nil {
		failed = 1
	}
	_ = mi.metrics.host.Emit(ctx, []extensionapi.Sample{
		{Metric: mi.metrics.DNSLookups, Value: 1, Tags: tags},
		{Metric: mi.metrics.DNSLookupDuration, Value: float64(duration.Milliseconds()), Tags: tags},
		{Metric: mi.metrics.DNSLookupFailed, Value: failed, Tags: tags},
	})
}

type moduleInstanceMetrics struct {
	host                  extensionapi.Metrics
	DNSResolutions        extensionapi.Metric
	DNSResolutionDuration extensionapi.Metric
	DNSResolutionFailed   extensionapi.Metric
	DNSLookups            extensionapi.Metric
	DNSLookupDuration     extensionapi.Metric
	DNSLookupFailed       extensionapi.Metric
}
