package tcp

import (
	"sort"

	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
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
	tcpConnecting *metrics.Metric
	tcpResolving  *metrics.Metric
	tcpDuration   *metrics.Metric

	tcpSockets       *metrics.Metric
	tcpReads         *metrics.Metric
	tcpWrites        *metrics.Metric
	tcpErrors        *metrics.Metric
	tcpTimeouts      *metrics.Metric
	tcpPartialWrites *metrics.Metric
}

func newTCPMetrics(vu modules.VU) *tcpMetrics {
	return &tcpMetrics{
		tcpConnecting:    vu.InitEnv().Registry.MustNewMetric(tcpConnecting, metrics.Trend, metrics.Time),
		tcpResolving:     vu.InitEnv().Registry.MustNewMetric(tcpResolving, metrics.Trend, metrics.Time),
		tcpDuration:      vu.InitEnv().Registry.MustNewMetric(tcpDuration, metrics.Trend, metrics.Time),
		tcpSockets:       vu.InitEnv().Registry.MustNewMetric(tcpSockets, metrics.Counter),
		tcpReads:         vu.InitEnv().Registry.MustNewMetric(tcpReads, metrics.Counter),
		tcpWrites:        vu.InitEnv().Registry.MustNewMetric(tcpWrites, metrics.Counter),
		tcpErrors:        vu.InitEnv().Registry.MustNewMetric(tcpErrors, metrics.Counter),
		tcpTimeouts:      vu.InitEnv().Registry.MustNewMetric(tcpTimeouts, metrics.Counter),
		tcpPartialWrites: vu.InitEnv().Registry.MustNewMetric(tcpPartialWrites, metrics.Counter),
	}
}

func addToTagSet(ts *metrics.TagSet, tags map[string]string) *metrics.TagSet {
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
