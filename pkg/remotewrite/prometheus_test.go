package remotewrite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	prompb "go.buf.build/grpc/go/prometheus/prometheus"
	"go.k6.io/k6/metrics"
)

func TestMapSeries(t *testing.T) {
	r := metrics.NewRegistry()
	series := metrics.TimeSeries{
		Metric: &metrics.Metric{
			Name: "test",
			Type: metrics.Counter,
		},
		Tags: r.RootTagSet().With("tagk1", "tagv1").With("b1", "v1"),
	}

	lbls := MapSeries(series)
	require.Len(t, lbls, 3)

	exp := []*prompb.Label{
		{Name: "__name__", Value: "k6_test"},
		{Name: "b1", Value: "v1"},
		{Name: "tagk1", Value: "tagv1"},
	}
	assert.Equal(t, exp, lbls)
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

func TestTrendAsGaugesFindIxName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		// they have to be sorted
		labels   []string
		expIndex uint16
	}{
		{
			labels:   []string{"tag1", "tag2"},
			expIndex: 0,
		},
		{
			labels:   []string{"2", "__name__"},
			expIndex: 1,
		},
		{
			labels:   []string{"__name__", "tag1", "__name__"},
			expIndex: 0,
		},
		{
			labels:   []string{"1", "__name__", "__name__1"},
			expIndex: 1,
		},
	}
	for _, tc := range cases {
		lbls := make([]*prompb.Label, 0, len(tc.labels))
		for _, l := range tc.labels {
			lbls = append(lbls, &prompb.Label{Name: l})
		}
		tg := trendAsGauges{labels: lbls}
		tg.FindNameIndex()
		assert.Equal(t, tc.expIndex, tg.ixname)
	}
}

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
