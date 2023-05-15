package metrics

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

var (
	_ Sink = &CounterSink{}
	_ Sink = &GaugeSink{}
	_ Sink = &TrendSink{}
	_ Sink = &RateSink{}
	_ Sink = &DummySink{}
)

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
		sink = &TrendSink{}
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

type GaugeSink struct {
	Value    float64
	Max, Min float64
	minSet   bool
}

// IsEmpty indicates whether the GaugeSink is empty.
func (g *GaugeSink) IsEmpty() bool { return !g.minSet }

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

func (g *GaugeSink) Format(t time.Duration) map[string]float64 {
	return map[string]float64{"value": g.Value}
}

type TrendSink struct {
	Values []float64
	sorted bool

	Count    uint64
	Min, Max float64
	Sum, Avg float64
}

// IsEmpty indicates whether the TrendSink is empty.
func (t *TrendSink) IsEmpty() bool { return t.Count == 0 }

func (t *TrendSink) Add(s Sample) {
	if t.Count == 0 {
		t.Max, t.Min = s.Value, s.Value
	} else {
		if s.Value > t.Max {
			t.Max = s.Value
		}
		if s.Value < t.Min {
			t.Min = s.Value
		}
	}

	t.Values = append(t.Values, s.Value)
	t.sorted = false
	t.Count += 1
	t.Sum += s.Value
	t.Avg = t.Sum / float64(t.Count)
}

// P calculates the given percentile from sink values.
func (t *TrendSink) P(pct float64) float64 {
	switch t.Count {
	case 0:
		return 0
	case 1:
		return t.Values[0]
	default:
		if !t.sorted {
			sort.Float64s(t.Values)
			t.sorted = true
		}

		// If percentile falls on a value in Values slice, we return that value.
		// If percentile does not fall on a value in Values slice, we calculate (linear interpolation)
		// the value that would fall at percentile, given the values above and below that percentile.
		i := pct * (float64(t.Count) - 1.0)
		j := t.Values[int(math.Floor(i))]
		k := t.Values[int(math.Ceil(i))]
		f := i - math.Floor(i)
		return j + (k-j)*f
	}
}

func (t *TrendSink) Format(tt time.Duration) map[string]float64 {
	// TODO: respect the summaryTrendStats for REST API
	return map[string]float64{
		"min":   t.Min,
		"max":   t.Max,
		"avg":   t.Avg,
		"med":   t.P(0.5),
		"p(90)": t.P(0.90),
		"p(95)": t.P(0.95),
	}
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

type DummySink map[string]float64

// IsEmpty indicates whether the DummySink is empty.
func (d DummySink) IsEmpty() bool { return len(d) == 0 }

func (d DummySink) Add(s Sample) {
	panic(errors.New("you can't add samples to a dummy sink"))
}

func (d DummySink) Format(t time.Duration) map[string]float64 {
	return map[string]float64(d)
}
