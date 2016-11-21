package stats

import (
	"sort"
)

type Sink interface {
	Add(s Sample)
	Format() map[string]float64
}

type CounterSink struct {
	Value float64
}

func (c *CounterSink) Add(s Sample) {
	c.Value += s.Value
}

func (c *CounterSink) Format() map[string]float64 {
	return map[string]float64{"count": c.Value}
}

type GaugeSink struct {
	Value float64
}

func (g *GaugeSink) Add(s Sample) {
	g.Value = s.Value
}

func (g *GaugeSink) Format() map[string]float64 {
	return map[string]float64{"value": g.Value}
}

type TrendSink struct {
	Values []float64

	jumbled  bool
	count    uint64
	min, max float64
	sum, avg float64
	med      float64
}

func (t *TrendSink) Add(s Sample) {
	t.Values = append(t.Values, s.Value)
	t.jumbled = true
	t.count += 1
	t.sum += s.Value
	t.avg = t.sum / float64(t.count)

	if s.Value > t.max {
		t.max = s.Value
	}
	if s.Value < t.min || t.min == 0 {
		t.min = s.Value
	}
}

func (t *TrendSink) Format() map[string]float64 {
	if t.jumbled {
		sort.Float64s(t.Values)
		t.jumbled = false

		t.med = t.Values[t.count/2]
		if (t.count & 0x01) == 0 {
			t.med = (t.med + t.Values[(t.count/2)-1]) / 2
		}
	}

	return map[string]float64{"min": t.min, "max": t.max, "avg": t.avg, "med": t.med}
}

type RateSink struct {
	Trues int64
	Total int64
}

func (r *RateSink) Add(s Sample) {
	r.Total += 1
	if s.Value != 0 {
		r.Trues += 1
	}
}

func (r RateSink) Format() map[string]float64 {
	return map[string]float64{"rate": float64(r.Trues) / float64(r.Total)}
}
