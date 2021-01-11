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

package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/loadimpact/k6/stats"
)

func TestNewMetricFamilyTrend(t *testing.T) {
	trend := stats.New("name", stats.Trend, stats.Time)
	sink := trend.Sink.(*stats.TrendSink)
	sink.Min = 1
	sink.Max = 10
	sink.Avg = 5
	sink.Med = 4

	m := NewMetricFamily(trend, 0)
	assert.Len(t, m, 6)

	assert.Equal(t, "name_min", *m[0].Name)
	assert.Equal(t, float64(1), *m[0].GetMetric()[0].Gauge.Value)

	assert.Equal(t, "name_max", *m[1].Name)
	assert.Equal(t, float64(10), *m[1].GetMetric()[0].Gauge.Value)

	assert.Equal(t, "name_avg", *m[2].Name)
	assert.Equal(t, float64(5), *m[2].GetMetric()[0].Gauge.Value)

	assert.Equal(t, "name_med", *m[3].Name)
	assert.Equal(t, float64(4), *m[3].GetMetric()[0].Gauge.Value)

	assert.Equal(t, "name_p90", *m[4].Name)
	assert.Equal(t, "name_p95", *m[5].Name)
}

func TestNewMetricFamilyCounter(t *testing.T) {
	counter := stats.New("name", stats.Counter, stats.Time)
	sink := counter.Sink.(*stats.CounterSink)
	sink.Value = 42

	m := NewMetricFamily(counter, 0)
	assert.Len(t, m, 2)

	assert.Equal(t, "name_count", *m[0].Name)
	assert.Equal(t, float64(42), *m[0].GetMetric()[0].Counter.Value)

	assert.Equal(t, "name_rate", *m[1].Name)
}

func TestNewMetricFamilyGauge(t *testing.T) {
	gauge := stats.New("name", stats.Gauge, stats.Time)
	sink := gauge.Sink.(*stats.GaugeSink)
	sink.Value = 42

	m := NewMetricFamily(gauge, 0)
	assert.Len(t, m, 1)

	assert.Equal(t, "name_value", *m[0].Name)
	assert.Equal(t, float64(42), *m[0].GetMetric()[0].Gauge.Value)
}

func TestNewMetricFamilyRate(t *testing.T) {
	rate := stats.New("name", stats.Rate, stats.Time)
	sink := rate.Sink.(*stats.RateSink)
	sink.Total = 42
	sink.Trues = 42 * 42

	m := NewMetricFamily(rate, 0)
	assert.Len(t, m, 1)

	assert.Equal(t, "name_rate", *m[0].Name)
	assert.Equal(t, float64(42), *m[0].GetMetric()[0].Gauge.Value)
}
