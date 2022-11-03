package remotewrite

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	prompb "go.buf.build/grpc/go/prometheus/prometheus"
	"go.k6.io/k6/metrics"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestExtendedTrendSinkMapPrompb(t *testing.T) {
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

	st := newExtendedTrendSink()
	st.Add(sample)
	require.Equal(t, st.Count, uint64(1))

	ts := st.MapPrompb(sample.TimeSeries, sample.Time)
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
		tg.CacheNameIndex()
		assert.Equal(t, tc.expIndex, tg.ixname)
	}
}

func TestNativeHistogramSinkAdd(t *testing.T) {
	t.Parallel()

	ts := metrics.TimeSeries{
		Metric: &metrics.Metric{
			Name:     "k6_test_metric",
			Contains: metrics.Time,
		},
	}
	sink := newNativeHistogramSink(ts.Metric)

	// k6 passes time values with ms time unit
	// the sink converts them to seconds.
	sink.Add(metrics.Sample{TimeSeries: ts, Value: float64((3 * time.Second).Milliseconds())})
	sink.Add(metrics.Sample{TimeSeries: ts, Value: float64((2 * time.Second).Milliseconds())})

	dmetric := &dto.Metric{}
	err := sink.H.Write(dmetric)
	require.NoError(t, err)

	assert.Equal(t, float64(5), *dmetric.Histogram.SampleSum)

	// the schema is generated from the bucket factor used
	assert.Equal(t, int32(3), *dmetric.Histogram.Schema)
}

func TestNativeHistogramSinkMapPrompb(t *testing.T) {
	t.Parallel()

	r := metrics.NewRegistry()
	series := metrics.TimeSeries{
		Metric: &metrics.Metric{
			Name: "test",
			Type: metrics.Trend,
		},
		Tags: r.RootTagSet().With("tagk1", "tagv1"),
	}

	st := newNativeHistogramSink(series.Metric)
	st.Add(metrics.Sample{
		TimeSeries: series,
		Value:      1.52,
		Time:       time.Unix(1, 0),
	})
	st.Add(metrics.Sample{
		TimeSeries: series,
		Value:      3.14,
		Time:       time.Unix(2, 0),
	})
	ts := st.MapPrompb(series, time.Unix(3, 0))

	// It should be the easiest way for asserting the entire struct,
	// because the structs contains a bunch of internals value that we don't want to assert.
	require.Len(t, ts, 1)
	b, err := protojson.Marshal(ts[0])
	require.NoError(t, err)

	expected := `{"labels":[{"name":"__name__","value":"k6_test"},{"name":"tagk1","value":"tagv1"}],"histograms":[{"countInt":"2","positiveDeltas":["1","0"],"positiveSpans":[{"length":1,"offset":5},{"length":1,"offset":8}],"schema":3,"sum":4.66,"timestamp":"3000","zeroCountInt":"0","zeroThreshold":2.938735877055719e-39}]}`
	assert.JSONEq(t, expected, string(b))
}
