package remotewrite

import (
	"sort"
	"testing"
	"time"

	prompb "go.buf.build/grpc/go/prometheus/prometheus"
	"go.k6.io/k6/metrics"
)

// buildTimeSeries creates a TimSeries with the given name, value and timestamp
func buildTimeSeries(name string, value float64, timestamp time.Time) *prompb.TimeSeries {
	return &prompb.TimeSeries{
		Labels: []*prompb.Label{
			{
				Name:  "__name__",
				Value: name,
			},
		},
		Samples: []*prompb.Sample{
			{
				Value:     value,
				Timestamp: timestamp.Unix(),
			},
		},
	}
}

// getTimeSeriesName returns the name of the time series defined in the '__name__' label
func getTimeSeriesName(ts *prompb.TimeSeries) string {
	for _, l := range ts.Labels {
		if l.Name == "__name__" {
			return l.Value
		}
	}
	return ""
}

// assertTimeSeriesEqual compares if two TimeSeries has the same name and value.
// Assumes only one sample per TimeSeries
func assertTimeSeriesEqual(t *testing.T, expected *prompb.TimeSeries, actual *prompb.TimeSeries) {
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
func sortTimeSeries(ts []*prompb.TimeSeries) []*prompb.TimeSeries {
	sorted := make([]*prompb.TimeSeries, len(ts))
	copy(sorted, ts)
	sort.Slice(sorted, func(i int, j int) bool {
		return getTimeSeriesName(sorted[i]) < getTimeSeriesName(sorted[j])
	})

	return sorted
}

// assertTimeSeriesMatch asserts if the elements of two arrays of TimeSeries match not considering order
func assertTimeSeriesMatch(t *testing.T, expected []*prompb.TimeSeries, actual []*prompb.TimeSeries) {
	if len(expected) != len(actual) {
		t.Errorf("timeseries length does not match. expected %d actual: %d", len(expected), len(actual))
	}

	// sort arrays
	se := sortTimeSeries(expected)
	sa := sortTimeSeries(actual)

	// return false if any element does not match
	for i := 0; i < len(se); i++ {
		assertTimeSeriesEqual(t, se[i], sa[i])
	}
}

func TestMapTrend(t *testing.T) {
	t.Parallel()

	now := time.Now()
	r := metrics.NewRegistry()

	testCases := []struct {
		sample   metrics.Sample
		labels   []prompb.Label
		expected []*prompb.TimeSeries
	}{
		{
			sample: metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: &metrics.Metric{
						Name: "test",
						Type: metrics.Trend,
					},
					Tags: r.RootTagSet().With("tagk1", "tagv1"),
				},
				Value: 1.0,
				Time:  now,
			},
			expected: []*prompb.TimeSeries{
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
		st := &trendSink{}
		st.Add(tc.sample)

		ts := MapTrend(tc.sample.TimeSeries, tc.sample.Time, st)
		assertTimeSeriesMatch(t, tc.expected, ts)
	}
}
