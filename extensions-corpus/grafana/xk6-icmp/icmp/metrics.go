package icmp

import (
	"fmt"

	extensionapi "go.k6.io/k6-extension-api"
)

const (
	icmpPacketsSent     = "icmp_packets_sent"
	icmpPacketsReceived = "icmp_packets_received"
	icmpReplyTTL        = "icmp_reply_ttl"
	icmpRtt             = "icmp_rtt"
	icmpResolve         = "icmp_resolve"
	icmpSetup           = "icmp_setup"
	icmpErrors          = "icmp_errors"
)

type icmpMetrics struct {
	host                extensionapi.Metrics
	dataSent            extensionapi.Metric
	dataReceived        extensionapi.Metric
	icmpPacketsSent     extensionapi.Metric
	icmpPacketsReceived extensionapi.Metric
	icmpReplyTTL        extensionapi.Metric
	icmpRtt             extensionapi.Metric
	icmpResolve         extensionapi.Metric
	icmpSetup           extensionapi.Metric
	icmpErrors          extensionapi.Metric
}

func newICMPMetrics(vu extensionapi.VU) *icmpMetrics {
	host, ok := vu.(extensionapi.Metrics)
	if !ok {
		panic("extension API metrics capability is unavailable")
	}
	register := func(name string, kind extensionapi.MetricKind, unit extensionapi.MetricUnit) extensionapi.Metric {
		metric, err := host.RegisterMetric(extensionapi.MetricSpec{Name: name, Kind: kind, Unit: unit})
		if err != nil {
			panic(fmt.Errorf("register ICMP metric %q: %w", name, err))
		}
		return metric
	}
	dataSent, ok := host.BuiltinMetric(extensionapi.BuiltinDataSent)
	if !ok {
		panic("extension API data_sent metric is unavailable")
	}
	dataReceived, ok := host.BuiltinMetric(extensionapi.BuiltinDataReceived)
	if !ok {
		panic("extension API data_received metric is unavailable")
	}
	return &icmpMetrics{
		host:                host,
		dataSent:            dataSent,
		dataReceived:        dataReceived,
		icmpPacketsSent:     register(icmpPacketsSent, extensionapi.MetricCounter, extensionapi.MetricUnitDefault),
		icmpPacketsReceived: register(icmpPacketsReceived, extensionapi.MetricCounter, extensionapi.MetricUnitDefault),
		icmpReplyTTL:        register(icmpReplyTTL, extensionapi.MetricGauge, extensionapi.MetricUnitDefault),
		icmpRtt:             register(icmpRtt, extensionapi.MetricTrend, extensionapi.MetricUnitTime),
		icmpResolve:         register(icmpResolve, extensionapi.MetricTrend, extensionapi.MetricUnitTime),
		icmpSetup:           register(icmpSetup, extensionapi.MetricTrend, extensionapi.MetricUnitTime),
		icmpErrors:          register(icmpErrors, extensionapi.MetricCounter, extensionapi.MetricUnitDefault),
	}
}
