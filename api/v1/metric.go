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
	"bytes"
	"encoding/json"
	"time"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/stats"
)

type NullMetricType struct {
	Type  stats.MetricType
	Valid bool
}

func (t NullMetricType) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return []byte("null"), nil
	}
	return t.Type.MarshalJSON()
}

func (t *NullMetricType) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		t.Valid = false
		return nil
	}
	t.Valid = true
	return json.Unmarshal(data, &t.Type)
}

type NullValueType struct {
	Type  stats.ValueType
	Valid bool
}

func (t NullValueType) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return []byte("null"), nil
	}
	return t.Type.MarshalJSON()
}

func (t *NullValueType) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		t.Valid = false
		return nil
	}
	t.Valid = true
	return json.Unmarshal(data, &t.Type)
}

type Metric struct {
	Name string `json:"-" yaml:"name"`

	Type     NullMetricType `json:"type" yaml:"type"`
	Contains NullValueType  `json:"contains" yaml:"contains"`
	Tainted  null.Bool      `json:"tainted" yaml:"tainted"`

	Sample map[string]float64 `json:"sample" yaml:"sample"`
}

type metricEnvelop struct {
	Data metricData `json:"data"`
}

type metricsEnvelop struct {
	Data []metricData `json:"data"`
}

type metricData struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes Metric `json:"attributes"`
}

func NewMetric(m *stats.Metric, t time.Duration) Metric {
	return Metric{
		Name:     m.Name,
		Type:     NullMetricType{m.Type, true},
		Contains: NullValueType{m.Contains, true},
		Tainted:  m.Tainted,
		Sample:   m.Sink.Format(t),
	}
}

// GetID TODO: delete
func (m Metric) GetID() string {
	return m.Name
}

// SetID TODO: delete
func (m *Metric) SetID(id string) error {
	m.Name = id
	return nil
}

func newMetricEnvelope(m *stats.Metric, t time.Duration) metricEnvelop {
	return metricEnvelop{
		Data: newMetricData(m, t),
	}
}

func newMetricsEnvelop(list map[string]*stats.Metric, t time.Duration) metricsEnvelop {
	metrics := make([]metricData, 0, len(list))

	// TODO: lock ?
	for _, m := range list {
		metrics = append(metrics, newMetricData(m, t))
	}

	return metricsEnvelop{
		Data: metrics,
	}
}

func newMetricData(m *stats.Metric, t time.Duration) metricData {
	metric := NewMetric(m, t)

	return metricData{
		Type:       "metrics",
		ID:         metric.Name,
		Attributes: metric,
	}
}
