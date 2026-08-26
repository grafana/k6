package icmp

import (
	"sort"

	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
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
	dataSent            *metrics.Metric
	dataReceived        *metrics.Metric
	icmpPacketsSent     *metrics.Metric
	icmpPacketsReceived *metrics.Metric
	icmpReplyTTL        *metrics.Metric
	icmpRtt             *metrics.Metric
	icmpResolve         *metrics.Metric
	icmpSetup           *metrics.Metric
	icmpErrors          *metrics.Metric
}

func newIcmpMetrics(vu modules.VU) *icmpMetrics {
	return &icmpMetrics{
		dataSent:            vu.InitEnv().BuiltinMetrics.DataSent,
		dataReceived:        vu.InitEnv().BuiltinMetrics.DataReceived,
		icmpPacketsSent:     vu.InitEnv().Registry.MustNewMetric(icmpPacketsSent, metrics.Counter),
		icmpPacketsReceived: vu.InitEnv().Registry.MustNewMetric(icmpPacketsReceived, metrics.Counter),
		icmpReplyTTL:        vu.InitEnv().Registry.MustNewMetric(icmpReplyTTL, metrics.Gauge),
		icmpRtt:             vu.InitEnv().Registry.MustNewMetric(icmpRtt, metrics.Trend, metrics.Time),
		icmpResolve:         vu.InitEnv().Registry.MustNewMetric(icmpResolve, metrics.Trend, metrics.Time),
		icmpSetup:           vu.InitEnv().Registry.MustNewMetric(icmpSetup, metrics.Trend, metrics.Time),
		icmpErrors:          vu.InitEnv().Registry.MustNewMetric(icmpErrors, metrics.Counter),
	}
}

func addToTagSet(ts *metrics.TagSet, tags map[string]string) *metrics.TagSet {
	if tags == nil {
		return ts
	}

	keys := make([]string, 0, len(tags))

	for k := range tags {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		ts = ts.With(k, tags[k])
	}

	return ts
}
