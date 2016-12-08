/*

k6 - a next-generation load testing tool
Copyright (C) 2016 Load Impact

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.

*/

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

func (t *TrendSink) P(pct float64) float64 {
	switch t.count {
	case 0:
		return 0
	case 1:
		return t.Values[0]
	case 2:
		if pct < 0.5 {
			return t.Values[0]
		} else {
			return t.Values[1]
		}
	default:
		return t.Values[int(float64(t.count)*pct)]
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

	return map[string]float64{
		"min": t.min,
		"max": t.max,
		"avg": t.avg,
		"med": t.med,
		"p90": t.P(0.90),
		"p95": t.P(0.95),
	}
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
