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
	value float64
	first time.Time
}

func (c *CounterSink) Add(s Sample) {
	c.value += s.Value
	if c.first.IsZero() {
		c.first = s.Time
	}
}

// LastValue returns the counter value.
func (c *CounterSink) LastValue() float64 {
	return c.value
}

// IsEmpty indicates whether the CounterSink is empty.
func (c *CounterSink) IsEmpty() bool { return c.first.IsZero() }

// Rate computes the rate per second of the aggregated values.
func (c *CounterSink) Rate(t time.Duration) float64 {
	if t == 0 {
		return 0
	}
	return c.value / (float64(t) / float64(time.Second))
}

type GaugeSink struct {
	value              float64
	maxValue, minValue float64
	minSet             bool
}

// IsEmpty indicates whether the GaugeSink is empty.
func (g *GaugeSink) IsEmpty() bool { return !g.minSet }

func (g *GaugeSink) Add(s Sample) {
	g.value = s.Value
	if s.Value > g.maxValue {
		g.maxValue = s.Value
	}
	if s.Value < g.minValue || !g.minSet {
		g.minValue = s.Value
		g.minSet = true
	}
}

// LastValue returns the Gauge current value.
func (g *GaugeSink) LastValue() float64 {
	return g.value
}

// Min returns the minimum observed value.
func (g *GaugeSink) Min() float64 {
	return g.minValue
}

// Max returns the maximum observed value.
func (g *GaugeSink) Max() float64 {
	return g.maxValue
}

type TrendSink struct {
	values []float64
	sorted bool

	countValue         uint64
	minValue, maxValue float64
	sumValue, avgValue float64
}

// IsEmpty indicates whether the TrendSink is empty.
func (t *TrendSink) IsEmpty() bool { return t.countValue == 0 }

func (t *TrendSink) Add(s Sample) {
	t.values = append(t.values, s.Value)
	t.sorted = false
	t.countValue++
	t.sumValue += s.Value
	t.avgValue = t.sumValue / float64(t.countValue)

	if s.Value > t.maxValue {
		t.maxValue = s.Value
	}
	if s.Value < t.minValue || t.countValue == 1 {
		t.minValue = s.Value
	}
}

// Avg returns the average value.
func (t *TrendSink) Avg() float64 {
	return t.avgValue
}

// Min returns the minimum observed value.
func (t *TrendSink) Min() float64 {
	return t.minValue
}

// Max returns the maximum observed value.
func (t *TrendSink) Max() float64 {
	return t.maxValue
}

// Count returns the amount of observed values.
func (t *TrendSink) Count() uint64 {
	return t.countValue
}

// Sum returns the sum of the observed values.
func (t *TrendSink) Sum() float64 {
	return t.sumValue
}

// P calculates the given percentile from sink values.
func (t *TrendSink) P(pct float64) float64 {
	switch t.countValue {
	case 0:
		return 0
	case 1:
		return t.values[0]
	default:
		if !t.sorted {
			sort.Float64s(t.values)
			t.sorted = true
		}

		// If percentile falls on a value in Values slice, we return that value.
		// If percentile does not fall on a value in Values slice, we calculate (linear interpolation)
		// the value that would fall at percentile, given the values above and below that percentile.
		i := pct * (float64(t.countValue) - 1.0)
		j := t.values[int(math.Floor(i))]
		k := t.values[int(math.Ceil(i))]
		f := i - math.Floor(i)
		return j + (k-j)*f
	}
}

type RateSink struct {
	trues int64
	total int64
}

// IsEmpty indicates whether the RateSink is empty.
func (r *RateSink) IsEmpty() bool { return r.total == 0 }

func (r *RateSink) Add(s Sample) {
	r.total++
	if s.Value != 0 {
		r.trues++
	}
}

// Rate computes the rate of non-zero aggregated values.
func (r *RateSink) Rate() float64 {
	var rate float64
	if r.total > 0 {
		rate = float64(r.trues) / float64(r.total)
	}
	return rate
}

// Count returns the amount of observed values.
func (r *RateSink) Count() uint64 {
	return uint64(r.total)
}

// NotZero returns the non zero values observed.
func (r *RateSink) NotZero() uint64 {
	return uint64(r.trues)
}

type DummySink map[string]float64

// IsEmpty indicates whether the DummySink is empty.
func (d DummySink) IsEmpty() bool { return len(d) == 0 }

func (d DummySink) Add(s Sample) {
	panic(errors.New("you can't add samples to a dummy sink"))
}
