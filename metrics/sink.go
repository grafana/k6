package metrics

import (
	"bytes"
	"fmt"
	"math"
	"time"

	"github.com/openhistogram/circonusllhist"
)

var (
	_ Sink = &CounterSink{}
	_ Sink = &GaugeSink{}
	_ Sink = NewTrendSink()
	_ Sink = &RateSink{}
)

type Sink interface {
	Add(s Sample)                              // Add a sample to the sink.
	Format(t time.Duration) map[string]float64 // Data for thresholds.
	IsEmpty() bool                             // Check if the Sink is empty.

	Drain() ([]byte, error) // Drain encodes the current sink values and clears them.
	Merge([]byte) error     // Merge decoeds the given values and merges them with the values in the current sink.
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

type CounterSink struct {
	Value float64
	First time.Time
}

func (c *CounterSink) Add(s Sample) {
	c.Value += s.Value
	if c.First.IsZero() {
		c.First = s.Time
	}
}

// IsEmpty indicates whether the CounterSink is empty.
func (c *CounterSink) IsEmpty() bool { return c.First.IsZero() }

func (c *CounterSink) Format(t time.Duration) map[string]float64 {
	return map[string]float64{
		"count": c.Value,
		"rate":  c.Value / (float64(t) / float64(time.Second)),
	}
}

// Drain encodes the current sink values and clears them.
// TODO: something more robust and efficient
func (c *CounterSink) Drain() ([]byte, error) {
	res := []byte(fmt.Sprintf("%d %b", c.First.UnixMilli(), c.Value))
	c.Value = 0
	return res, nil
}

// Merge decoeds the given values and merges them with the values in the current sink.
func (c *CounterSink) Merge(from []byte) error {
	var firstMs int64
	var val float64
	_, err := fmt.Sscanf(string(from), "%d %b", &firstMs, &val)
	if err != nil {
		return err
	}

	c.Value += val
	if first := time.UnixMilli(firstMs); c.First.After(first) {
		c.First = first
	}

	return nil
}

type GaugeSink struct {
	Last     time.Time
	Value    float64
	Max, Min float64
	minSet   bool
}

// IsEmpty indicates whether the GaugeSink is empty.
func (g *GaugeSink) IsEmpty() bool { return !g.minSet }

func (g *GaugeSink) Add(s Sample) {
	g.Last = s.Time
	g.Value = s.Value
	if s.Value > g.Max {
		g.Max = s.Value
	}
	if s.Value < g.Min || !g.minSet {
		g.Min = s.Value
		g.minSet = true
	}
}

func (g *GaugeSink) Format(t time.Duration) map[string]float64 {
	return map[string]float64{"value": g.Value}
}

// Drain encodes the current sink values and clears them.
//
// TODO: something more robust and efficient
func (g *GaugeSink) Drain() ([]byte, error) {
	res := []byte(fmt.Sprintf("%d %b %b %b", g.Last.UnixMilli(), g.Value, g.Min, g.Max))

	g.Last = time.Time{}
	g.Value = 0

	return res, nil
}

// Merge decoeds the given values and merges them with the values in the current sink.
func (g *GaugeSink) Merge(from []byte) error {
	var lastMms int64
	var val, min, max float64
	_, err := fmt.Sscanf(string(from), "%d %b %b %b", &lastMms, &val, &min, &max)
	if err != nil {
		return err
	}

	last := time.UnixMilli(lastMms)
	if last.After(g.Last) {
		g.Last = last
		g.Value = val
	}

	if max > g.Max {
		g.Max = max
	}
	if min < g.Min || !g.minSet {
		g.Min = min
		g.minSet = true
	}

	return nil
}

// NewTrendSink makes a Trend sink with the OpenHistogram circllhist histogram.
func NewTrendSink() *TrendSink {
	return &TrendSink{
		hist: circonusllhist.New(circonusllhist.NoLocks()),
	}
}

// TrendSink uses the OpenHistogram circllhist histogram to store metrics data.
type TrendSink struct {
	hist *circonusllhist.Histogram
}

func (t *TrendSink) nanToZero(val float64) float64 {
	if math.IsNaN(val) {
		return 0
	}
	return val
}

// IsEmpty indicates whether the TrendSink is empty.
func (t *TrendSink) IsEmpty() bool { return t.hist.Count() == 0 }

// Add records the given sample value in the HDR histogram.
func (t *TrendSink) Add(s Sample) {
	// TODO: handle the error, log something when there's an error
	_ = t.hist.RecordValue(s.Value)
}

// Min returns the approximate minimum value from the histogram.
func (t *TrendSink) Min() float64 {
	return t.nanToZero(t.hist.Min())
}

// Max returns the approximate maximum value from the histogram.
func (t *TrendSink) Max() float64 {
	return t.nanToZero(t.hist.Max())
}

// Count returns the number of recorded values.
func (t *TrendSink) Count() uint64 {
	return t.hist.Count()
}

// Avg returns the approximate average (i.e. mean) value from the histogram.
func (t *TrendSink) Avg() float64 {
	return t.nanToZero(t.hist.ApproxMean())
}

// Total returns the approximate total (i.e. "sum") value for all measurements.
func (t *TrendSink) Total() float64 {
	return t.nanToZero(t.hist.ApproxSum())
}

// P calculates the given percentile from sink values.
func (t *TrendSink) P(pct float64) float64 {
	return t.nanToZero(t.hist.ValueAtQuantile(pct))
}

func (t *TrendSink) Format(tt time.Duration) map[string]float64 {
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

// Drain encodes the current sink values and clears them.
func (t *TrendSink) Drain() ([]byte, error) {
	b := &bytes.Buffer{} // TODO: reuse buffers?
	if err := t.hist.Serialize(b); err != nil {
		return nil, err
	}
	t.hist.Reset()
	return b.Bytes(), nil
}

// Merge decoeds the given values and merges them with the values in the current sink.
func (t *TrendSink) Merge(from []byte) error {
	b := bytes.NewBuffer(from)

	hist, err := circonusllhist.DeserializeWithOptions(
		b, circonusllhist.NoLocks(), // TODO: investigate circonusllhist.NoLookup
	)
	if err != nil {
		return err
	}

	t.hist.Merge(hist)
	return nil
}

type RateSink struct {
	Trues int64
	Total int64
}

// IsEmpty indicates whether the RateSink is empty.
func (r *RateSink) IsEmpty() bool { return r.Total == 0 }

func (r *RateSink) Add(s Sample) {
	r.Total += 1
	if s.Value != 0 {
		r.Trues += 1
	}
}

func (r RateSink) Format(t time.Duration) map[string]float64 {
	var rate float64
	if r.Total > 0 {
		rate = float64(r.Trues) / float64(r.Total)
	}

	return map[string]float64{"rate": rate}
}

// Drain encodes the current sink values and clears them.
//
// TODO: something more robust and efficient
func (r *RateSink) Drain() ([]byte, error) {
	res := []byte(fmt.Sprintf("%d %d", r.Trues, r.Total))
	r.Trues = 0
	r.Total = 0
	return res, nil
}

// Merge decoeds the given values and merges them with the values in the current sink.
func (r *RateSink) Merge(from []byte) error {
	var trues, total int64
	_, err := fmt.Sscanf(string(from), "%d %d", &trues, &total)
	if err != nil {
		return err
	}

	r.Trues += trues
	r.Total += total
	return nil
}
