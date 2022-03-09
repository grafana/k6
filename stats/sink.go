/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package stats

import (
	"errors"
	"math"
	"time"

	"github.com/openhistogram/circonusllhist"
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

func (r RateSink) Format(t time.Duration) map[string]float64 {
	return map[string]float64{"rate": float64(r.Trues) / float64(r.Total)}
}

type DummySink map[string]float64

func (d DummySink) Add(s Sample) {
	panic(errors.New("you can't add samples to a dummy sink"))
}

func (d DummySink) Format(t time.Duration) map[string]float64 {
	return map[string]float64(d)
}
