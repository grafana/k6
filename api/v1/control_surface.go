package v1

import (
	"context"

	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

type metricsObserver interface {
	ObservedMetrics() []*metrics.Metric
	ObservedMetricByName(string) (*metrics.Metric, bool)
}

type breachedThresholdCounter interface {
	GetMetricsWithBreachedThresholdsCount() uint32
}

// TODO: Refactor by removing this interface and split on two different fields.
// At this level, it isn't really required to know the internal detail
// that metricsEngine handles both metrics and thresholds.
// They can stay as two separated entity even if they are the same under the hood.
type metricsAndThresholdsObserver interface {
	metricsObserver
	breachedThresholdCounter
}

// ControlSurface includes the methods the REST API can use to control and
// communicate with the rest of k6.
type ControlSurface struct {
	RunCtx        context.Context
	Samples       chan metrics.SampleContainer
	MetricsEngine metricsAndThresholdsObserver
	Scheduler     *execution.Scheduler
	RunState      *lib.TestRunState
}
