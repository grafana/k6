package metrics

import (
	"errors"
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

// Sink collects Sample's values.
type Sink interface {
	// Add adds a sample to the sink.
	Add(s Sample)

	// IsEmpty checks if the Sink has never received a value.
	IsEmpty() bool
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

// Rate computes the rate per second of the aggregated values.
func (c *CounterSink) Rate(t time.Duration) float64 {
	if t == 0 {
		return 0
	}
	return c.Value / (float64(t) / float64(time.Second))
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
	t.Values = append(t.Values, s.Value)
	t.sorted = false
	t.Count += 1
	t.Sum += s.Value
	t.Avg = t.Sum / float64(t.Count)

	if s.Value > t.Max {
		t.Max = s.Value
	}
	if s.Value < t.Min || t.Count == 1 {
		t.Min = s.Value
	}
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

// Rate computes the rate of non-zero aggregated values.
func (r RateSink) Rate() float64 {
	var rate float64
	if r.Total > 0 {
		rate = float64(r.Trues) / float64(r.Total)
	}
	return rate
}

type DummySink map[string]float64

// IsEmpty indicates whether the DummySink is empty.
func (d DummySink) IsEmpty() bool { return len(d) == 0 }

func (d DummySink) Add(s Sample) {
	panic(errors.New("you can't add samples to a dummy sink"))
}
