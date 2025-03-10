package metrics

import (
	"math"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// HdrHistogramSink is a Sink implementation that relies
// on a HdrHistogram under that hood. So, it is less accurate
// but also has less impact in terms of memory allocations.
type HdrHistogramSink struct {
	hdr *hdrhistogram.Histogram
}

// NewHdrHistogramSink instantiates a new HdrHistogramSink
// with values between [1, 100000000000] with the number
// of significant value digits up to 5.
func NewHdrHistogramSink() HdrHistogramSink {
	return HdrHistogramSink{
		hdr: hdrhistogram.New(
			0,
			math.MaxInt64,
			3,
		),
	}
}

// IsEmpty indicates whether the TrendSink is empty.
func (h HdrHistogramSink) IsEmpty() bool { return h.hdr.TotalCount() == 0 }

// Add implements the Sink interface, recording the value of the given Sample.
func (h HdrHistogramSink) Add(s Sample) {
	// FIXME: Handle error
	_ = h.hdr.RecordValue(int64(s.Value))
}

// P implements the Sink interface, returning the value at percentile.
func (h HdrHistogramSink) P(pct float64) float64 {
	return float64(h.hdr.ValueAtPercentile(pct * 100.0))
}

// Min implements the Sink interface, returning the minimum value recorded.
func (h HdrHistogramSink) Min() float64 {
	return float64(h.hdr.Min())
}

// Max implements the Sink interface, returning the maximum value recorded.
func (h HdrHistogramSink) Max() float64 {
	return float64(h.hdr.Max())
}

// Count implements the Sink interface, returning the total amount of values recorded.
func (h HdrHistogramSink) Count() uint64 {
	return uint64(h.hdr.TotalCount())
}

// Avg implements the Sink interface, returning the average (mean) of values recorded.
func (h HdrHistogramSink) Avg() float64 {
	return h.hdr.Mean()
}

// Format trend and return a map
func (h HdrHistogramSink) Format(_ time.Duration) map[string]float64 {
	return map[string]float64{
		"min":   h.Min(),
		"max":   h.Max(),
		"avg":   h.Avg(),
		"med":   h.P(0.5),
		"p(90)": h.P(0.90),
		"p(95)": h.P(0.95),
	}
}

// Merge merges two Sink instances.
func (h HdrHistogramSink) Merge(s Sink) {
	toMerge, ok := s.(HdrHistogramSink)
	if !ok {
		panic("trying to merge incompatible trend sinks")
	}

	// FIXME: Handle error
	_ = h.hdr.Merge(toMerge.hdr)
}

// We want to make sure that the HdrHistogramSink
// implements the Sink interface.
var _ Sink = HdrHistogramSink{}
