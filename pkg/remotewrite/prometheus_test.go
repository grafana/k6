package remotewrite

import (
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/metrics"
)

// check that ad-hoc optimization doesn't produce wrong values
func TestTrendAdd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		current  *metrics.Metric
		s        metrics.Sample
		expected metrics.TrendSink
	}{
		{
			current: &metrics.Metric{
				Sink: &metrics.TrendSink{},
			},
			s: metrics.Sample{Value: 2},
			expected: metrics.TrendSink{
				Values: []float64{2},
				Count:  1,
				Min:    2,
				Max:    2,
				Sum:    2,
				Avg:    2,
				Med:    2,
			},
		},
		{
			current: &metrics.Metric{
				Sink: &metrics.TrendSink{
					Values: []float64{8, 3, 1, 7, 4, 2},
					Count:  6,
					Min:    1,
					Max:    8,
					Sum:    25,
				},
			},
			s: metrics.Sample{Value: 12.3},
			expected: metrics.TrendSink{
				Values: []float64{8, 3, 1, 7, 4, 2, 12.3},
				Count:  7,
				Min:    1,
				Max:    12.3,
				Sum:    37.3,
				Avg:    37.3 / 7,
				Med:    7,
			},
		},
	}

	for _, testCase := range testCases {
		// trendAdd should result in the same values as Sink.Add

		trendAdd(testCase.current, testCase.s)
		sink := testCase.current.Sink.(*metrics.TrendSink)

		assert.Equal(t, testCase.expected.Count, sink.Count)
		assert.Equal(t, testCase.expected.Min, sink.Min)
		assert.Equal(t, testCase.expected.Max, sink.Max)
		assert.Equal(t, testCase.expected.Sum, sink.Sum)
		assert.Equal(t, testCase.expected.Avg, sink.Avg)
		assert.Equal(t, testCase.expected.Med, sink.Med)
		assert.Equal(t, testCase.expected.Values, sink.Values)
	}
}

func BenchmarkTrendAdd(b *testing.B) {
	benchF := []func(b *testing.B, start metrics.Metric){
		func(b *testing.B, m metrics.Metric) {
			b.ResetTimer()
			rand.Seed(time.Now().Unix())

			for i := 0; i < b.N; i++ {
				trendAdd(&m, metrics.Sample{Value: rand.Float64() * 1000})
				sink := m.Sink.(*metrics.TrendSink)
				p(sink, 0.90)
				p(sink, 0.95)
			}
		},
		func(b *testing.B, start metrics.Metric) {
			b.ResetTimer()
			rand.Seed(time.Now().Unix())

			for i := 0; i < b.N; i++ {
				start.Sink.Add(metrics.Sample{Value: rand.Float64() * 1000})
				start.Sink.Format(0)
			}
		},
	}

	start := metrics.Metric{
		Type: metrics.Trend,
		Sink: &metrics.TrendSink{},
	}

	b.Run("trendAdd", func(b *testing.B) {
		benchF[0](b, start)
	})
	b.Run("TrendSink.Add", func(b *testing.B) {
		benchF[1](b, start)
	})
}

// buildTimeSeries creates a TimSeries with the given name, value and timestamp
func buildTimeSeries(name string, value float64, timestamp time.Time) prompb.TimeSeries {
	return prompb.TimeSeries{
		Labels: []prompb.Label{
			{
				Name:  "__name__",
				Value: name,
			},
		},
		Samples: []prompb.Sample{
			{
				Value:     value,
				Timestamp: timestamp.Unix(),
			},
		},
	}
}

// getTimeSeriesName returs the name of the timeseries defined in the '__name__' label
func getTimeSeriesName(ts prompb.TimeSeries) string {
	for _, l := range ts.Labels {
		if l.Name == "__name__" {
			return l.Value
		}
	}
	return ""
}

// assertTimeSeriesEqual compares if two TimeSeries has the same name and value.
// Assumes only one sample per TimeSeries
func assertTimeSeriesEqual(t *testing.T, expected prompb.TimeSeries, actual prompb.TimeSeries) {
	expectedName := getTimeSeriesName(expected)
	actualName := getTimeSeriesName(actual)
	if expectedName != actualName {
		t.Errorf("names do not match expected: %s actual: %s", expectedName, actualName)
	}

	expectedValue := expected.Samples[0].Value
	actualValue := actual.Samples[0].Value
	if expectedValue != actualValue {
		t.Errorf("values do not match expected: %f actual: %f", expectedValue, actualValue)
	}
}

// sortTimeSeries sorts an array of TimeSeries by name
func sortTimeSeries(ts []prompb.TimeSeries) []prompb.TimeSeries {
	sorted := make([]prompb.TimeSeries, len(ts))
	copy(sorted, ts)
	sort.Slice(sorted, func(i int, j int) bool {
		return getTimeSeriesName(sorted[i]) < getTimeSeriesName(sorted[j])
	})

	return sorted
}

// assertTimeSeriesMatch asserts if the elements of two arrays of TimeSeries match not considering order
func assertTimeSeriesMatch(t *testing.T, expected []prompb.TimeSeries, actual []prompb.TimeSeries) {
	if len(expected) != len(actual) {
		t.Errorf("timeseries length does not match. expected %d actual: %d", len(expected), len(actual))
	}

	//sort arrays
	se := sortTimeSeries(expected)
	sa := sortTimeSeries(actual)

	//return false if any element does not match
	for i := 0; i < len(se); i++ {
		assertTimeSeriesEqual(t, se[i], sa[i])
	}

}

func TestMapTrend(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testCases := []struct {
		storage  *metricsStorage
		sample   metrics.Sample
		labels   []prompb.Label
		expected []prompb.TimeSeries
	}{
		{
			storage: newMetricsStorage(),
			sample: metrics.Sample{
				Metric: &metrics.Metric{
					Name: "test",
					Type: metrics.Trend,
				},
				Value: 1.0,
				Time:  now,
			},
			expected: []prompb.TimeSeries{
				buildTimeSeries("k6_test_count", 1.0, now),
				buildTimeSeries("k6_test_sum", 1.0, now),
				buildTimeSeries("k6_test_min", 1.0, now),
				buildTimeSeries("k6_test_max", 1.0, now),
				buildTimeSeries("k6_test_avg", 1.0, now),
				buildTimeSeries("k6_test_med", 1.0, now),
				buildTimeSeries("k6_test_p90", 1.0, now),
				buildTimeSeries("k6_test_p95", 1.0, now),
			},
		},
	}

	for _, tc := range testCases {
		m := &PrometheusMapping{}
		ts := m.MapTrend(tc.storage, tc.sample, tc.labels)
		assertTimeSeriesMatch(t, tc.expected, ts)
	}
}
