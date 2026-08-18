package tcp

import (
	"fmt"

	extensionapi "go.k6.io/k6-extension-api"
)

const (
	tcpConnecting = "tcp_socket_connecting"
	tcpResolving  = "tcp_socket_resolving"
	tcpDuration   = "tcp_socket_duration"

	tcpSockets       = "tcp_sockets"
	tcpReads         = "tcp_reads"
	tcpWrites        = "tcp_writes"
	tcpErrors        = "tcp_errors"
	tcpTimeouts      = "tcp_timeouts"
	tcpPartialWrites = "tcp_partial_writes"
)

type tcpMetrics struct {
	host          extensionapi.Metrics
	tcpConnecting extensionapi.Metric
	tcpResolving  extensionapi.Metric
	tcpDuration   extensionapi.Metric

	tcpSockets       extensionapi.Metric
	tcpReads         extensionapi.Metric
	tcpWrites        extensionapi.Metric
	tcpErrors        extensionapi.Metric
	tcpTimeouts      extensionapi.Metric
	tcpPartialWrites extensionapi.Metric
}

func newTCPMetrics(vu extensionapi.VU) *tcpMetrics {
	host, ok := vu.(extensionapi.Metrics)
	if !ok {
		panic("extension API metrics capability is unavailable")
	}
	register := func(name string, kind extensionapi.MetricKind, unit extensionapi.MetricUnit) extensionapi.Metric {
		metric, err := host.RegisterMetric(extensionapi.MetricSpec{Name: name, Kind: kind, Unit: unit})
		if err != nil {
			panic(fmt.Errorf("register TCP metric %q: %w", name, err))
		}
		return metric
	}
	return &tcpMetrics{host: host,
		tcpConnecting:    register(tcpConnecting, extensionapi.MetricTrend, extensionapi.MetricUnitTime),
		tcpResolving:     register(tcpResolving, extensionapi.MetricTrend, extensionapi.MetricUnitTime),
		tcpDuration:      register(tcpDuration, extensionapi.MetricTrend, extensionapi.MetricUnitTime),
		tcpSockets:       register(tcpSockets, extensionapi.MetricCounter, extensionapi.MetricUnitDefault),
		tcpReads:         register(tcpReads, extensionapi.MetricCounter, extensionapi.MetricUnitDefault),
		tcpWrites:        register(tcpWrites, extensionapi.MetricCounter, extensionapi.MetricUnitDefault),
		tcpErrors:        register(tcpErrors, extensionapi.MetricCounter, extensionapi.MetricUnitDefault),
		tcpTimeouts:      register(tcpTimeouts, extensionapi.MetricCounter, extensionapi.MetricUnitDefault),
		tcpPartialWrites: register(tcpPartialWrites, extensionapi.MetricCounter, extensionapi.MetricUnitDefault)}
}
