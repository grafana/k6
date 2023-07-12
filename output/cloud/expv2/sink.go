package expv2

import (
	"fmt"

	"go.k6.io/k6/metrics"
)

// TODO:A potential optimization
// https://github.com/grafana/k6/pull/3085#discussion_r1210415981
type metricValue interface {
	Add(v float64)
}

func newMetricValue(mt metrics.MetricType) metricValue {
	var am metricValue
	switch mt {
	case metrics.Counter:
		am = &counter{}
	case metrics.Gauge:
		am = &gauge{}
	case metrics.Rate:
		am = &rate{}
	case metrics.Trend:
		am = newHistogram()
	default:
		// Should not be possible to create
		// an invalid metric type except for specific
		// and controlled tests
		panic(fmt.Sprintf("MetricType %q is not supported", mt))
	}
	return am
}

type counter struct {
	Sum float64
}

func (c *counter) Add(v float64) {
	c.Sum += v
}

type gauge struct {
	Last     float64
	Sum      float64
	Min, Max float64
	Avg      float64
	Count    uint32

	minSet bool
}

func (g *gauge) Add(v float64) {
	g.Last = v
	g.Count++
	g.Sum += v
	g.Avg = g.Sum / float64(g.Count)

	if v > g.Max {
		g.Max = v
	}
	if !g.minSet || v < g.Min {
		g.Min = v
		g.minSet = true
	}
}

type rate struct {
	NonZeroCount uint32
	Total        uint32
}

func (r *rate) Add(v float64) {
	r.Total++
	if v != 0 {
		r.NonZeroCount++
	}
}
