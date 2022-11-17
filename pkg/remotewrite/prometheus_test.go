package remotewrite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				Timestamp: timestamp.UnixMilli(),
			},
		},
	}
}

// assertTimeSeriesMatch asserts if the elements of two slices of TimeSeries matches.
func assertTimeSeriesEqual(t *testing.T, expected []*prompb.TimeSeries, actual []*prompb.TimeSeries) {
	t.Helper()
	require.Len(t, actual, len(expected))

	for i := 0; i < len(expected); i++ {
		assert.Equal(t, expected[i], actual[i])
	}
}

func TestMapTrendAsGauges(t *testing.T) {
	t.Parallel()

	now := time.Now()
	r := metrics.NewRegistry()

	sample := metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: &metrics.Metric{
				Name: "test",
				Type: metrics.Trend,
			},
			Tags: r.RootTagSet(),
		},
		Value: 1.0,
		Time:  now,
	}

	expected := []*prompb.TimeSeries{
		buildTimeSeries("k6_test_count", 1.0, now),
		buildTimeSeries("k6_test_sum", 1.0, now),
		buildTimeSeries("k6_test_min", 1.0, now),
		buildTimeSeries("k6_test_max", 1.0, now),
		buildTimeSeries("k6_test_avg", 1.0, now),
		buildTimeSeries("k6_test_med", 1.0, now),
		buildTimeSeries("k6_test_p90", 1.0, now),
		buildTimeSeries("k6_test_p95", 1.0, now),
	}

	st := &metrics.TrendSink{}
	st.Add(sample)
	require.Equal(t, st.Count, uint64(1))

	ts := MapTrendAsGauges(sample.TimeSeries, sample.Time, st)
	require.Len(t, ts, 8)
	assertTimeSeriesEqual(t, expected, ts)
}

// TODO: the sorting logic for labels must be moved and tested as a shared
// and centralized concept.
func TestMapTrendAsGaugesMustBeSorted(t *testing.T) {
	r := metrics.NewRegistry()
	series := metrics.TimeSeries{
		Metric: &metrics.Metric{
			Name: "test",
			Type: metrics.Trend,
		},
		Tags: r.RootTagSet().With("tagk1", "tagv1").With("b1", "v1"),
	}

	sample := metrics.Sample{
		TimeSeries: series,
		Value:      1.52,
		Time:       time.Unix(1, 0),
	}
	st := &metrics.TrendSink{}
	st.Add(sample)

	ts := MapTrendAsGauges(sample.TimeSeries, sample.Time, st)
	require.Len(t, ts, 8)
	require.Len(t, ts[0].Labels, 3)
	assert.Equal(t, []string{"__name__", "b1", "tagk1"}, func() []string {
		labels := make([]string, 0, len(ts[0].Labels))
		for _, l := range ts[0].Labels {
			labels = append(labels, l.Name)
		}
		return labels
	}())
}
