package sse

import (
	"errors"

	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
)

// MetricEventName is the sse event metric of the module
const MetricEventName = "sse_event"

type sseMetrics struct {
	SSEEventReceived *metrics.Metric
}

// registerMetrics registers the metrics for the sse module in the metrics registry
func registerMetrics(vu modules.VU) (sseMetrics, error) {
	var err error
	m := sseMetrics{}
	env := vu.InitEnv()
	if env == nil {
		return m, errors.New("missing env")
	}
	registry := env.Registry
	if registry == nil {
		return m, errors.New("missing registry")
	}

	m.SSEEventReceived, err = registry.NewMetric(MetricEventName, metrics.Counter)
	if err != nil {
		return m, err
	}

	return m, nil
}
