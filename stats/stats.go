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
	"strconv"
	"time"

	"strings"

	"github.com/dustin/go-humanize"
	"gopkg.in/guregu/null.v3"
)

const (
	counterString = `"counter"`
	gaugeString   = `"gauge"`
	trendString   = `"trend"`
	rateString    = `"rate"`

	defaultString = `"default"`
	timeString    = `"time"`
	dataString    = `"data"`
)

// Possible values for MetricType.
const (
	Counter = MetricType(iota) // A counter that sums its data points
	Gauge                      // A gauge that displays the latest value
	Trend                      // A trend, min/max/avg/med are interesting
	Rate                       // A rate, displays % of values that aren't 0
)

// Possible values for ValueType.
const (
	Default = ValueType(iota) // Values are presented as-is
	Time                      // Values are timestamps (nanoseconds)
	Data                      // Values are data amounts (bytes)
)

// The serialized metric type is invalid.
var ErrInvalidMetricType = errors.New("Invalid metric type")

// The serialized value type is invalid.
var ErrInvalidValueType = errors.New("Invalid value type")

// A MetricType specifies the type of a metric.
type MetricType int

// MarshalJSON serializes a MetricType as a human readable string.
func (t MetricType) MarshalJSON() ([]byte, error) {
	switch t {
	case Counter:
		return []byte(counterString), nil
	case Gauge:
		return []byte(gaugeString), nil
	case Trend:
		return []byte(trendString), nil
	case Rate:
		return []byte(rateString), nil
	default:
		return nil, ErrInvalidMetricType
	}
}

// UnmarshalJSON deserializes a MetricType from a string representation.
func (t *MetricType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case counterString:
		*t = Counter
	case gaugeString:
		*t = Gauge
	case trendString:
		*t = Trend
	case rateString:
		*t = Rate
	default:
		return ErrInvalidMetricType
	}

	return nil
}

func (t MetricType) String() string {
	switch t {
	case Counter:
		return counterString
	case Gauge:
		return gaugeString
	case Trend:
		return trendString
	case Rate:
		return rateString
	default:
		return "[INVALID]"
	}
}

// The type of values a metric contains.
type ValueType int

// MarshalJSON serializes a ValueType as a human readable string.
func (t ValueType) MarshalJSON() ([]byte, error) {
	switch t {
	case Default:
		return []byte(defaultString), nil
	case Time:
		return []byte(timeString), nil
	case Data:
		return []byte(dataString), nil
	default:
		return nil, ErrInvalidValueType
	}
}

// UnmarshalJSON deserializes a ValueType from a string representation.
func (t *ValueType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case defaultString:
		*t = Default
	case timeString:
		*t = Time
	case dataString:
		*t = Data
	default:
		return ErrInvalidValueType
	}

	return nil
}

func (t ValueType) String() string {
	switch t {
	case Default:
		return defaultString
	case Time:
		return timeString
	case Data:
		return dataString
	default:
		return "[INVALID]"
	}
}

// A Sample is a single measurement.
type Sample struct {
	Metric *Metric
	Time   time.Time
	Tags   map[string]string
	Value  float64
}

// A Metric defines the shape of a set of data.
type Metric struct {
	Name       string       `json:"name"`
	Type       MetricType   `json:"type"`
	Contains   ValueType    `json:"contains"`
	Tainted    null.Bool    `json:"tainted"`
	Thresholds Thresholds   `json:"thresholds"`
	Submetrics []*Submetric `json:"submetrics"`
	Sub        Submetric    `json:"sub,omitempty"`
	Sink       Sink         `json:"-"`
}

func New(name string, typ MetricType, t ...ValueType) *Metric {
	vt := Default
	if len(t) > 0 {
		vt = t[0]
	}
	var sink Sink
	switch typ {
	case Counter:
		sink = &CounterSink{}
	case Gauge:
		sink = &GaugeSink{}
	case Trend:
		sink = &TrendSink{}
	case Rate:
		sink = &RateSink{}
	default:
		return nil
	}
	return &Metric{Name: name, Type: typ, Contains: vt, Sink: sink}
}

func (m *Metric) HumanizeValue(v float64) string {
	switch m.Type {
	case Rate:
		return strconv.FormatFloat(100*v, 'f', 2, 64) + "%"
	default:
		switch m.Contains {
		case Time:
			d := ToD(v)
			switch {
			case d > time.Minute:
				d -= d % (1 * time.Second)
			case d > time.Second:
				d -= d % (10 * time.Millisecond)
			case d > time.Millisecond:
				d -= d % (10 * time.Microsecond)
			case d > time.Microsecond:
				d -= d % (10 * time.Nanosecond)
			}
			return d.String()
		case Data:
			return humanize.Bytes(uint64(v))
		default:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
	}
}

// A Submetric represents a filtered dataset based on a parent metric.
type Submetric struct {
	Name   string            `json:"name"`
	Parent string            `json:"parent"`
	Suffix string            `json:"suffix"`
	Tags   map[string]string `json:"tags"`
	Metric *Metric           `json:"metric"`
}

// Creates a submetric from a name.
func NewSubmetric(name string) (parentName string, sm *Submetric) {
	parts := strings.SplitN(strings.TrimSuffix(name, "}"), "{", 2)
	if len(parts) == 1 {
		return parts[0], &Submetric{Name: name}
	}

	kvs := strings.Split(parts[1], ",")
	tags := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, ":", 2)

		key := strings.TrimSpace(strings.Trim(parts[0], `"'`))
		if len(parts) != 2 {
			tags[key] = ""
			continue
		}

		value := strings.TrimSpace(strings.Trim(parts[1], `"'`))
		tags[key] = value
	}
	return parts[0], &Submetric{Name: name, Parent: parts[0], Suffix: parts[1], Tags: tags}
}

func (m *Metric) Summary() *Summary {
	return &Summary{
		Metric:  m,
		Summary: m.Sink.Format(),
	}
}

type Summary struct {
	Metric  *Metric            `json:"metric"`
	Summary map[string]float64 `json:"summary"`
}
