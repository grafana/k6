package v2

import (
	"bytes"
	"encoding/json"
	"github.com/loadimpact/k6/stats"
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
}

func NewMetric(m stats.Metric) Metric {
	return Metric{
		Name:     m.Name,
		Type:     NullMetricType{m.Type, true},
		Contains: NullValueType{m.Contains, true},
	}
}

func (m Metric) GetID() string {
	return m.Name
}

func (m *Metric) SetID(id string) error {
	m.Name = id
	return nil
}
