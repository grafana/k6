package v1

import (
	"bytes"
	"encoding/json"
	"time"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/metrics"
)

type NullMetricType struct {
	Type  metrics.MetricType
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
	Type  metrics.ValueType
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

// NewMetric constructs a new Metric
func NewMetric(swm *metrics.SinkWithMetric, t time.Duration) Metric {
	return Metric{
		Name:     swm.Metric.Name,
		Type:     NullMetricType{swm.Metric.Type, true},
		Contains: NullValueType{swm.Metric.Contains, true},
		Tainted:  swm.Metric.Tainted,
		Sample:   swm.Sink.Format(t),
	}
}
