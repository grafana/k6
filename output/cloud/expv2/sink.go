package expv2

import (
	"go.k6.io/k6/metrics"
)

func newSink(mt metrics.MetricType) metrics.Sink {
	if mt == metrics.Trend {
		return &histogram{}
	}

	return metrics.NewSink(mt)
}
