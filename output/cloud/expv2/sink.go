package expv2

import (
	"time"

	"go.k6.io/k6/metrics"
)

func newSink(mt metrics.MetricType) metrics.Sink {
	if mt == metrics.Trend {
		return &histogram{}
	}

	return metrics.NewSink(mt)
}

// TODO: implement the HDR histogram
type histogram struct{}

func (h *histogram) IsEmpty() bool                           { return true }
func (h *histogram) Add(metrics.Sample)                      {}
func (h *histogram) Format(time.Duration) map[string]float64 { panic("nyi") }
