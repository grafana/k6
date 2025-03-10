package metrics

import (
	"fmt"
	"time"

	"go.k6.io/k6/internal/ds/histogram"
)

var (
	_ Sink = &CounterSink{}
	_ Sink = &GaugeSink{}
	_ Sink = NewTrendSink()
	_ Sink = &RateSink{}
)

// Sink is a sample sink which will accumulate data in specific way
type Sink interface {
	Add(s Sample)                              // Add a sample to the sink.
	Format(t time.Duration) map[string]float64 // Data for thresholds.
	IsEmpty() bool                             // Check if the Sink is empty.
}

// NewSink creates the related Sink for
// the provided MetricType.
func NewSink(mt MetricType) Sink {
	var sink Sink
	switch mt {
	case Counter:
		sink = &CounterSink{}
	case Gauge:
		sink = &GaugeSink{}
	case Trend:
		sink = NewTrendSink()
	case Rate:
		sink = &RateSink{}
	default:
		// Should not be possible to create
		// an invalid metric type except for specific
		// and controlled tests
		panic(fmt.Sprintf("MetricType %q is not supported", mt))
	}
	return sink
}

// CounterSink is a sink that represents a Counter
type CounterSink struct {
	Value float64
	First time.Time
}

// Add a single sample to the sink
func (c *CounterSink) Add(s Sample) {
	c.Value += s.Value
	if c.First.IsZero() {
		c.First = s.Time
	}
}

// IsEmpty indicates whether the CounterSink is empty.
func (c *CounterSink) IsEmpty() bool { return c.First.IsZero() }

// Format counter and return a map
func (c *CounterSink) Format(t time.Duration) map[string]float64 {
	return map[string]float64{
		"count": c.Value,
		"rate":  c.Value / (float64(t) / float64(time.Second)),
	}
}

// GaugeSink is a sink represents a Gauge
type GaugeSink struct {
	Value    float64
	Max, Min float64
	minSet   bool
}

// IsEmpty indicates whether the GaugeSink is empty.
func (g *GaugeSink) IsEmpty() bool { return !g.minSet }

// Add a single sample to the sink
func (g *GaugeSink) Add(s Sample) {
	g.Value = s.Value
	if s.Value > g.Max {
		g.Max = s.Value
	}
	if s.Value < g.Min || !g.minSet {
		g.Min = s.Value
		g.minSet = true
	}
}

// Format gauge and return a map
func (g *GaugeSink) Format(_ time.Duration) map[string]float64 {
	return map[string]float64{"value": g.Value}
}

// NewTrendSink makes a trend Sink with an HDR histogram-based implementation.
func NewTrendSink() *TrendSink {
	return &TrendSink{
		h: histogram.NewHdr(),
	}
}

// TrendSink is a sink for a Trend
type TrendSink struct {
	h *histogram.Hdr
}

// IsEmpty indicates whether the TrendSink is empty.
func (t *TrendSink) IsEmpty() bool { return t.h.Count == 0 }

// Add a single sample into the trend
func (t *TrendSink) Add(s Sample) {
	t.h.Add(s.Value)
}

// P calculates the given percentile from sink values.
func (t *TrendSink) P(pct float64) float64 {
	switch t.h.Count {
	case 0:
		return 0
	case 1:
		return t.h.Min
	default:
		return t.h.Quantile(pct)
	}
}

// Min returns the minimum value.
func (t *TrendSink) Min() float64 {
	if t.IsEmpty() {
		return 0
	}
	return t.h.Min
}

// Max returns the maximum value.
func (t *TrendSink) Max() float64 {
	if t.IsEmpty() {
		return 0
	}
	return t.h.Max
}

// Count returns the number of recorded values.
func (t *TrendSink) Count() uint64 {
	return uint64(t.h.Count)
}

// Avg returns the average (i.e. mean) value.
func (t *TrendSink) Avg() float64 {
	if t.IsEmpty() {
		return 0
	}
	return t.h.Sum / float64(t.h.Count)
}

// Total returns the total (i.e. "sum") value for all measurements.
func (t *TrendSink) Total() float64 {
	return t.h.Sum
}

// Format trend and return a map
func (t *TrendSink) Format(_ time.Duration) map[string]float64 {
	// TODO: respect the summaryTrendStats for REST API
	return map[string]float64{
		"min":   t.Min(),
		"max":   t.Max(),
		"avg":   t.Avg(),
		"med":   t.P(0.5),
		"p(90)": t.P(0.90),
		"p(95)": t.P(0.95),
	}
}

// RateSink is a sink for a rate
type RateSink struct {
	Trues int64
	Total int64
}

// IsEmpty indicates whether the RateSink is empty.
func (r *RateSink) IsEmpty() bool { return r.Total == 0 }

// Add a single sample to the rate
func (r *RateSink) Add(s Sample) {
	r.Total++
	if s.Value != 0 {
		r.Trues++
	}
}

// Format rate and return a map
func (r RateSink) Format(_ time.Duration) map[string]float64 {
	var rate float64
	if r.Total > 0 {
		rate = float64(r.Trues) / float64(r.Total)
	}

	return map[string]float64{"rate": rate}
}
