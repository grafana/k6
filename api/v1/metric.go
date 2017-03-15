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

	"github.com/loadimpact/k6/stats"
	"gopkg.in/guregu/null.v3"
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
	Name string `json:"-"`

	Type     NullMetricType `json:"type"`
	Contains NullValueType  `json:"contains"`
	Tainted  null.Bool      `json:"tainted"`

	Sample map[string]float64 `json:"sample"`
}

func NewMetric(m stats.Metric, sink stats.Sink) Metric {
	return Metric{
		Name:     m.Name,
		Type:     NullMetricType{m.Type, true},
		Contains: NullValueType{m.Contains, true},
		Tainted:  m.Tainted,
		Sample:   sink.Format(),
	}
}

func (m Metric) GetID() string {
	return m.Name
}

func (m *Metric) SetID(id string) error {
	m.Name = id
	return nil
}
