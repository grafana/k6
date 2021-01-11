/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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
	"bytes"
	"time"

	"github.com/loadimpact/k6/stats"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func newMetricFamily(m *stats.Metric, t time.Duration) []dto.MetricFamily {
	metrics := make([]dto.MetricFamily, 0)

	switch m.Type {
	case stats.Counter:
		counter := m.Sink.(*stats.CounterSink)
		metrics = append(metrics, newCounter(m.Name+"_count", counter.Value, m.Name+" cumulative value"))
		metrics = append(metrics, newGauge(m.Name+"_rate", counter.Value/(float64(t)/float64(time.Second)),
			m.Name+" value per seconds"))
	case stats.Gauge:
		gauge := m.Sink.(*stats.GaugeSink)
		metrics = append(metrics, newGauge(m.Name+"_value", gauge.Value, m.Name+" latest value"))
	case stats.Trend:
		trend := m.Sink.(*stats.TrendSink)
		trend.Calc()
		metrics = append(metrics, newGauge(m.Name+"_min", trend.Min, m.Name+" minimum value"))
		metrics = append(metrics, newGauge(m.Name+"_max", trend.Max, m.Name+" maximum value"))
		metrics = append(metrics, newGauge(m.Name+"_avg", trend.Avg, m.Name+" average value"))
		metrics = append(metrics, newGauge(m.Name+"_med", trend.Med, m.Name+" median value"))
		metrics = append(metrics, newGauge(m.Name+"_p90", trend.P(0.90), m.Name+" 90 percentile value"))
		metrics = append(metrics, newGauge(m.Name+"_p95", trend.P(0.95), m.Name+" 95 percentile value"))
	case stats.Rate:
		rate := m.Sink.(*stats.RateSink)
		metrics = append(metrics, newGauge(m.Name+"_rate", float64(rate.Trues)/float64(rate.Total),
			m.Name+" percentage of non-zero values"))
	}
	return metrics
}

func newGauge(name string, value float64, help string) dto.MetricFamily {
	return dto.MetricFamily{
		Name:   &name,
		Help:   &help,
		Type:   dto.MetricType_GAUGE.Enum(),
		Metric: []*dto.Metric{{Gauge: &dto.Gauge{Value: &value}}},
	}
}

func newCounter(name string, value float64, help string) dto.MetricFamily {
	return dto.MetricFamily{
		Name:   &name,
		Help:   &help,
		Type:   dto.MetricType_COUNTER.Enum(),
		Metric: []*dto.Metric{{Counter: &dto.Counter{Value: &value}}},
	}
}

func marshallMetricFamily(metrics []dto.MetricFamily) ([]byte, error) {
	var b bytes.Buffer
	for i := range metrics {
		_, err := expfmt.MetricFamilyToText(&b, &metrics[i])
		if err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}
