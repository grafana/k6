package metrics

import "errors"

// A MetricType specifies the type of a metric.
type MetricType int

// Possible values for MetricType.
const (
	Counter = MetricType(iota) // A counter that sums its data points
	Gauge                      // A gauge that displays the latest value
	Trend                      // A trend, min/max/avg/med are interesting
	Rate                       // A rate, displays % of values that aren't 0
)

// ErrInvalidMetricType indicates the serialized metric type is invalid.
var ErrInvalidMetricType = errors.New("invalid metric type")

const (
	counterString = "counter"
	gaugeString   = "gauge"
	trendString   = "trend"
	rateString    = "rate"

	defaultString = "default"
	timeString    = "time"
	dataString    = "data"
)

// MarshalJSON serializes a MetricType as a human readable string.
func (t MetricType) MarshalJSON() ([]byte, error) {
	txt, err := t.MarshalText()
	if err != nil {
		return nil, err
	}
	return []byte(`"` + string(txt) + `"`), nil
}

// MarshalText serializes a MetricType as a human readable string.
func (t MetricType) MarshalText() ([]byte, error) {
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

// UnmarshalText deserializes a MetricType from a string representation.
func (t *MetricType) UnmarshalText(data []byte) error {
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

// supportedAggregationMethods returns the list of threshold aggregation methods
// that can be used against this MetricType.
func (t MetricType) supportedAggregationMethods() []string {
	switch t {
	case Counter:
		return []string{tokenCount, tokenRate}
	case Gauge:
		return []string{tokenValue}
	case Rate:
		return []string{tokenRate}
	case Trend:
		return []string{
			tokenAvg,
			tokenMin,
			tokenMax,
			tokenMed,
			tokenPercentile,
		}
	default:
		// unreachable!
		panic("unreachable")
	}
}

// supportsAggregationMethod returns whether the MetricType supports a
// given threshold aggregation method or not.
func (t MetricType) supportsAggregationMethod(aggregationMethod string) bool {
	for _, m := range t.supportedAggregationMethods() {
		if aggregationMethod == m {
			return true
		}
	}

	return false
}
