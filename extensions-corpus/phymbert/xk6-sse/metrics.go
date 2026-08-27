package sse

import (
	"errors"

	"go.k6.io/k6-extension-api"
)

// MetricEventName is the sse event metric of the module
const MetricEventName = "sse_event"

type sseMetrics struct {
	SSEEventReceived extensionapi.Metric
}

// registerMetrics registers the metrics for the sse module in the metrics registry
func registerMetrics(vu extensionapi.VU) (sseMetrics, error) {
	metrics, ok := vu.(extensionapi.Metrics)
	if !ok {
		return sseMetrics{}, errors.New("extension API metrics capability is unavailable")
	}

	metric, err := metrics.RegisterMetric(extensionapi.MetricSpec{
		Name: MetricEventName,
		Kind: extensionapi.MetricCounter,
	})
	if err != nil {
		return sseMetrics{}, err
	}

	return sseMetrics{SSEEventReceived: metric}, nil
}
