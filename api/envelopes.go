package api

import (
	"errors"
	"github.com/loadimpact/speedboat/stats"
)

var (
	ErrInvalidMetricType = errors.New("Invalid metric type")
	ErrInvalidValueType  = errors.New("Invalid value type")
)

const (
	counterString = `"counter"`
	gaugeString   = `"gauge"`
	trendString   = `"trend"`

	defaultString = `"default"`
	timeString    = `"time"`
)

type Error struct {
	Title string `json:"title"`
}

type ErrorResponse struct {
	Errors []Error `json:"errors"`
}

type MetricType stats.MetricType

func (t *MetricType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case counterString:
		*t = MetricType(stats.Counter)
	case gaugeString:
		*t = MetricType(stats.Gauge)
	case trendString:
		*t = MetricType(stats.Trend)
	default:
		return ErrInvalidMetricType
	}

	return nil
}

func (t MetricType) MarshalJSON() ([]byte, error) {
	switch stats.MetricType(t) {
	case stats.Counter:
		return []byte(counterString), nil
	case stats.Gauge:
		return []byte(gaugeString), nil
	case stats.Trend:
		return []byte(trendString), nil
	default:
		return nil, ErrInvalidMetricType
	}
}

type ValueType stats.ValueType

func (t *ValueType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case defaultString:
		*t = ValueType(stats.Default)
	case timeString:
		*t = ValueType(stats.Time)
	default:
		return ErrInvalidValueType
	}

	return nil
}

func (t ValueType) MarshalJSON() ([]byte, error) {
	switch stats.ValueType(t) {
	case stats.Default:
		return []byte(defaultString), nil
	case stats.Time:
		return []byte(timeString), nil
	default:
		return nil, ErrInvalidValueType
	}
}

type Metric struct {
	Name     string             `json:"name"`
	Type     MetricType         `json:"type"`
	Contains ValueType          `json:"contains"`
	Data     map[string]float64 `json:"data"`
}

type MetricSet map[string]Metric
