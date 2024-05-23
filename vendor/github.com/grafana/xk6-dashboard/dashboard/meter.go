// SPDX-FileCopyrightText: 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

import (
	"math"
	"sort"
	"strings"
	"time"

	"go.k6.io/k6/metrics"
)

type meter struct {
	registry *registry

	clock  *metrics.GaugeSink
	period time.Duration
	start  time.Time
	tags   []string
}

func newMeter(period time.Duration, now time.Time, tags []string) *meter {
	registry := newRegistry()
	metric := registry.mustGetOrNew("time", metrics.Gauge, metrics.Time, nil)
	clock, _ := metric.Sink.(*metrics.GaugeSink)

	start := now
	clock.Add(metrics.Sample{Time: now, Value: float64(start.UnixMilli())})

	return &meter{
		registry: registry,
		start:    start,
		clock:    clock,
		period:   period,
		tags:     tags,
	}
}

func (m *meter) toSnapshot(period time.Duration, now time.Time) *meter {
	meter := newMeter(period, now, m.tags)

	for _, met := range m.registry.All() {
		if meter.registry.Get(met.Name) != nil {
			continue
		}

		clone, _ := meter.registry.getOrNew(
			met.Name,
			met.Type,
			met.Contains,
			thresholdsSources(met.Thresholds),
		)

		for _, sub := range met.Submetrics {
			clone.AddSubmetric(sub.Suffix) //nolint:errcheck,gosec
		}
	}

	return meter
}

func (m *meter) update(containers []metrics.SampleContainer, now time.Time) ([]sampleData, error) {
	dur := m.period
	if dur == 0 {
		dur = now.Sub(m.start)
	}

	m.clock.Value = float64(now.UnixMilli())

	for _, container := range containers {
		for _, sample := range container.GetSamples() {
			if err := m.add(sample); err != nil {
				return nil, err
			}
		}
	}

	return m.format(dur), nil
}

func (m *meter) add(sample metrics.Sample) error {
	metric, err := m.registry.getOrNew(
		sample.Metric.Name,
		sample.Metric.Type,
		sample.Metric.Contains,
		thresholdsSources(sample.Metric.Thresholds),
	)
	if err != nil {
		return err
	}

	metric.Sink.Add(sample)

	if sample.Tags == nil {
		return nil
	}

	for _, tag := range m.tags {
		val, ok := sample.Tags.Get(tag)
		if !ok || len(val) == 0 {
			continue
		}

		sub, err := metric.AddSubmetric(tag + ":" + val)
		if err != nil {
			return err
		}

		sub.Metric.Sink.Add(sample)
	}

	return nil
}

func (m *meter) format(dur time.Duration) []sampleData {
	fmt := func(met *metrics.Metric) sampleData {
		if met.Sink.IsEmpty() {
			return sampleData{}
		}

		sample := met.Sink.Format(dur)

		if sink, ok := met.Sink.(*metrics.TrendSink); ok {
			sample[pc99Name] = sink.P(pc99)
		}

		for name, value := range sample {
			sample[name] = significant(value)
		}

		names := aggregateNames(met.Type)

		data := make([]float64, 0, len(names))

		for _, name := range names {
			data = append(data, sample[name])
		}

		return data
	}

	out := make(map[string]sampleData, len(m.registry.names))
	names := make([]string, 0, len(m.registry.names))

	for _, name := range m.registry.names {
		metric := m.registry.Get(name)
		if metric == nil {
			continue
		}

		out[name] = fmt(metric)

		names = append(names, metric.Name)

		for _, sub := range metric.Submetrics {
			out[sub.Name] = fmt(sub.Metric)
			names = append(names, sub.Name)
		}
	}

	sort.Strings(names)

	arr := make([]sampleData, 0, len(out))

	for _, name := range names {
		arr = append(arr, out[name])
	}

	return arr
}

func (m *meter) evaluate(now time.Time) map[string][]string {
	failures := make(map[string][]string)

	dur := m.period
	if dur == 0 {
		dur = now.Sub(m.start)
	}

	for _, name := range m.registry.names {
		metric := m.registry.Get(name)
		if metric == nil {
			continue
		}

		pass, err := metric.Thresholds.Run(metric.Sink, dur)
		if err != nil || !pass {
			srcs := make([]string, 0)

			for _, t := range metric.Thresholds.Thresholds {
				if t.LastFailed {
					srcs = append(srcs, t.Source)
				}
			}

			failures[name] = srcs
		}
	}

	return failures
}

func significant(num float64) float64 {
	const (
		ten1 = float64(10)
		ten2 = ten1 * 10
		ten3 = ten2 * 10
		ten4 = ten3 * 10
		ten5 = ten4 * 10
	)

	if num == float64(int(num)) {
		return num
	}

	if num > ten4 {
		return math.Trunc(num)
	}

	if num > ten3 {
		return math.Trunc(num*ten1) / ten1
	}

	if num > ten2 {
		return math.Trunc(num*ten2) / ten2
	}

	if num > ten1 {
		return math.Trunc(num*ten3) / ten3
	}

	if num > 1 {
		return math.Trunc(num*ten4) / ten4
	}

	return math.Trunc(num*ten5) / ten5
}

func (m *meter) newbies(seen []string) (map[string]metricData, []string) {
	names, updated := m.registry.newbies(seen)
	if len(names) == 0 {
		return nil, updated
	}

	newbies := make(map[string]metricData, len(names))

	for _, name := range names {
		metric := m.get(name)
		if metric == nil {
			continue
		}

		newbies[name] = *newMetricData(metric)
	}

	return newbies, updated
}

func (m *meter) get(name string) *metrics.Metric {
	openingPos := strings.IndexByte(name, '{')
	if openingPos == -1 {
		return m.registry.Get(name)
	}

	metric := m.registry.Get(name[:openingPos])

	for _, sub := range metric.Submetrics {
		if sub.Name == name {
			return sub.Metric
		}
	}

	return metric
}

type metricData struct {
	Type     metrics.MetricType `json:"type"`
	Contains metrics.ValueType  `json:"contains,omitempty"`
	Tainted  bool               `json:"tainted,omitempty"`
	Custom   bool               `json:"custom,omitempty"`
}

func newMetricData(origin *metrics.Metric) *metricData {
	name := origin.Name
	openingPos := strings.IndexByte(name, '{')
	if openingPos != -1 {
		name = name[:openingPos]
	}

	return &metricData{
		Type:     origin.Type,
		Contains: origin.Contains,
		Tainted:  origin.Tainted.Bool,
		Custom:   !isBuiltin(name),
	}
}

type sampleData []float64

func thresholdsSources(thresholds metrics.Thresholds) []string {
	strs := make([]string, 0, len(thresholds.Thresholds))

	for _, t := range thresholds.Thresholds {
		strs = append(strs, t.Source)
	}

	return strs
}

const (
	pc99     = 0.99
	pc99Name = "p(99)"
)

func aggregateNames(mtype metrics.MetricType) []string {
	switch mtype {
	case metrics.Gauge:
		return gaugeAggregateNames
	case metrics.Rate:
		return rateAggregateNames
	case metrics.Counter:
		return counterAggregateNames
	case metrics.Trend:
		return trendAggregateNames
	default:
		return nil
	}
}

//nolint:gochecknoglobals
var (
	gaugeAggregateNames   = []string{"value"}
	rateAggregateNames    = []string{"rate"}
	counterAggregateNames = []string{"count", "rate"}
	trendAggregateNames   = []string{"avg", "max", "med", "min", "p(90)", "p(95)", "p(99)"}
)
