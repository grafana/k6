// Package dns provides a k6 module for DNS resolution and lookup.
package dns

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/js/promises"
	"go.k6.io/k6/v2/metrics"

	"github.com/grafana/sobek"
)

type (
	// RootModule is the module that will be registered with the runtime.
	RootModule struct{}

	// ModuleInstance is the module instance that will be created for each VU.
	ModuleInstance struct {
		vu        modules.VU
		dnsClient *Client
		metrics   *moduleInstanceMetrics
	}
)

// Ensure the interfaces are implemented correctly
var (
	_ modules.Instance = &ModuleInstance{}
	_ modules.Module   = &RootModule{}
)

// New creates a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance creates a new instance of the module for a specific VU.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	instanceMetrics, err := registerMetrics(metrics.NewRegistry())
	if err != nil {
		common.Throw(vu.Runtime(), fmt.Errorf("failed to register dns module instance's metrics; reason: %w", err))
	}

	dnsClient, err := NewDNSClient(vu)
	if err != nil {
		common.Throw(vu.Runtime(), fmt.Errorf("failed to create DNS client; reason: %w", err))
	}

	return &ModuleInstance{
		vu:        vu,
		dnsClient: dnsClient,
		metrics:   instanceMetrics,
	}
}

// Exports returns the module exports, that will be available in the runtime.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Named: map[string]any{
		"resolve": mi.Resolve,
		"lookup":  mi.Lookup,
	}}
}

// Resolve resolves a domain name to an IP address.
func (mi *ModuleInstance) Resolve(query, recordType, nameserverAddr sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(mi.vu)

	if mi.vu.State() == nil {
		reject(errors.New("resolve can not be used in the init context"))
		return promise
	}

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
		// Start timer for resolution
		resolutionStartTime := time.Now()

		// Resolve the query
		results, resolveErr := mi.dnsClient.Resolve(mi.vu.Context(), queryStr, recordTypeStr, nameserver)

		// Stop the timer for resolution
		sinceResolutionStart := time.Since(resolutionStartTime).Milliseconds()

		// Emit the metrics, regardless of the result
		mi.emitResolutionMetrics(
			mi.vu.Context(),
			sinceResolutionStart,
			queryStr,
			recordTypeStr,
			nameserver,
			resolveErr,
		)

		// Handle the resolution failure only now that we have emitted the metrics
		if resolveErr != nil {
			reject(resolveErr)
			return
		}

		resolve(results)
	}()

	return promise
}

// Lookup resolves a domain name to an IP address using the default system nameservers.
func (mi *ModuleInstance) Lookup(hostname sobek.Value) *sobek.Promise {
	promise, resolve, reject := promises.New(mi.vu)

	if mi.vu.State() == nil {
		reject(fmt.Errorf("lookup can not be used in the init context"))
		return promise
	}

	var hostnameStr string
	if err := mi.vu.Runtime().ExportTo(hostname, &hostnameStr); err != nil {
		reject(fmt.Errorf("hostname must be a string; got %T instead", hostname))
		return promise
	}

	go func() {
		// Start the timer for the lookup
		lookupStartTime := time.Now()

		// Perform the lookup
		ips, lookupErr := mi.dnsClient.Lookup(mi.vu.Context(), hostnameStr)

		// Stop the timer for the lookup
		sinceLookupStart := time.Since(lookupStartTime).Milliseconds()

		// Emit the metrics, regardless of the result
		mi.emitLookupMetrics(
			mi.vu.Context(),
			sinceLookupStart,
			hostnameStr,
			lookupErr,
		)

		// Handle the lookup failure only now that we have emitted the metrics
		if lookupErr != nil {
			reject(lookupErr)
			return
		}

		resolve(ips)
	}()

	return promise
}

// registerMetrics registers the metrics for the module instance.
func registerMetrics(registry *metrics.Registry) (*moduleInstanceMetrics, error) {
	var err error
	m := &moduleInstanceMetrics{}

	m.DNSResolutions, err = registry.NewMetric("dns_resolutions", metrics.Counter)
	if err != nil {
		return nil, fmt.Errorf("failed registering dns_resolutions metric: %w", err)
	}

	m.DNSResolutionDuration, err = registry.NewMetric("dns_resolution_duration", metrics.Trend, metrics.Time)
	if err != nil {
		return nil, fmt.Errorf("failed registering dns_resolution_duration metric: %w", err)
	}

	m.DNSResolutionFailed, err = registry.NewMetric("dns_resolution_failed", metrics.Rate)
	if err != nil {
		return nil, fmt.Errorf("failed registering dns_resolution_failed metric: %w", err)
	}

	m.DNSLookups, err = registry.NewMetric("dns_lookups", metrics.Counter)
	if err != nil {
		return nil, fmt.Errorf("failed registering dns_lookups metric: %w", err)
	}

	m.DNSLookupDuration, err = registry.NewMetric("dns_lookup_duration", metrics.Trend, metrics.Time)
	if err != nil {
		return nil, fmt.Errorf("failed registering dns_lookup_duration metric: %w", err)
	}

	m.DNSLookupFailed, err = registry.NewMetric("dns_lookup_failed", metrics.Rate)
	if err != nil {
		return nil, fmt.Errorf("failed registering dns_lookup_failed metric: %w", err)
	}

	return m, nil
}

// emitResolutionMetrics emits the metrics specific to DNS resolution operations.
func (mi *ModuleInstance) emitResolutionMetrics(
	ctx context.Context,
	duration int64,
	query,
	recordType string,
	nameserver Nameserver,
	resolutionErr error,
) {
	state := mi.vu.State()

	tags := state.Tags.GetCurrentValues().Tags
	tags = tags.With("query", query)
	tags = tags.With("recordType", recordType)
	tags = tags.With("nameserver", nameserver.Addr())

	now := time.Now()

	// Increment the DNS lookups counter
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: mi.metrics.DNSResolutions,
			Tags:   tags,
		},
		Time:     now,
		Metadata: nil,
		Value:    float64(1),
	})

	// Emit the DNS lookup duration
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: mi.metrics.DNSResolutionDuration,
			Tags:   tags,
		},
		Time:     now,
		Value:    float64(duration),
		Metadata: nil,
	})

	var failed float64
	if resolutionErr != nil {
		failed = 1
	}

	// Emit the DNS resolution failed rate
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: mi.metrics.DNSResolutionFailed,
			Tags:   tags,
		},
		Time:     now,
		Value:    failed,
		Metadata: nil,
	})
}

// emitLookupMetrics emits the metrics specific to DNS lookup operations.
func (mi *ModuleInstance) emitLookupMetrics(
	ctx context.Context,
	duration int64,
	host string,
	lookupErr error,
) {
	state := mi.vu.State()

	tags := state.Tags.GetCurrentValues().Tags
	tags = tags.With("host", host)

	now := time.Now()

	// Increment the DNS lookups counter
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: mi.metrics.DNSLookups,
			Tags:   tags,
		},
		Time:     now,
		Metadata: nil,
		Value:    float64(1),
	})

	// Emit the DNS lookup duration
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: mi.metrics.DNSLookupDuration,
			Tags:   tags,
		},
		Time:     now,
		Value:    float64(duration),
		Metadata: nil,
	})

	var failed float64
	if lookupErr != nil {
		failed = 1
	}

	// Emit the DNS lookup failed rate
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: mi.metrics.DNSLookupFailed,
			Tags:   tags,
		},
		Time:     now,
		Value:    failed,
		Metadata: nil,
	})
}

// moduleInstanceMetrics holds the metrics for the module instance.
type moduleInstanceMetrics struct {
	// DNSResolutions is a counter metric tracking the total number of DNS resolutions.
	DNSResolutions *metrics.Metric

	// DNSResolutionDuration is a trend metric tracking the duration of DNS resolutions.
	DNSResolutionDuration *metrics.Metric

	// DNSResolutionFailed is a Rate metric tracking the rate of failed DNS resolutions.
	DNSResolutionFailed *metrics.Metric

	// DNSLookups is a counter metric tracking the total number of DNS lookups.
	DNSLookups *metrics.Metric

	// DNSLookupDuration is a trend metric tracking the duration of DNS lookups.
	DNSLookupDuration *metrics.Metric

	// DNSLookupFailed is a Rate metric tracking the rate of failed DNS lookups.
	DNSLookupFailed *metrics.Metric
}
