package remotewrite

import (
	"math/rand"
	"testing"
	"time"

	prompb "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		buildTimeSeries("k6_test_avg", 1.0, now),
		buildTimeSeries("k6_test_count", 1.0, now),
		buildTimeSeries("k6_test_max", 1.0, now),
		buildTimeSeries("k6_test_med", 1.0, now),
		buildTimeSeries("k6_test_min", 1.0, now),
		buildTimeSeries("k6_test_p095", 1.0, now),
		buildTimeSeries("k6_test_p90", 1.0, now),
		buildTimeSeries("k6_test_sum", 1.0, now),
	}
	resolver, err := metrics.GetResolversForTrendColumns([]string{"count", "min", "max", "avg", "med", "p(90)", "p(95)"})
	require.NoError(t, err)
	resolver["p90"] = resolver["p(90)"]
	delete(resolver, "p(90)")
	resolver["p095"] = resolver["p(95)"]
	delete(resolver, "p(95)")
	resolver["sum"] = func(t *metrics.TrendSink) float64 {
		return t.Total()
	}

	st, err := newExtendedTrendSink(resolver)
	require.NoError(t, err)
	st.Add(sample)
	require.Equal(t, st.Count(), uint64(1))

	ts := st.MapPrompb(sample.TimeSeries, sample.Time)
	require.Len(t, ts, 8)

	sortByNameLabel(ts)
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

func BenchmarkK6TrendSinkAdd(b *testing.B) {
	m := &metrics.Metric{
		Type: metrics.Trend,
		Sink: metrics.NewTrendSink(),
	}
	s := metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: m,
		},
		Value: rand.Float64(), //nolint:gosec
		Time:  time.Now(),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Sink.Add(s)
	}
}

func TestNativeHistogramSinkMapPrompbWithValueType(t *testing.T) {
	t.Parallel()

	r := metrics.NewRegistry()
	series := metrics.TimeSeries{
		Metric: &metrics.Metric{
			Name:     "test",
			Type:     metrics.Trend,
			Contains: metrics.Time,
		},
		Tags: r.RootTagSet(),
	}

	st := newNativeHistogramSink(series.Metric)
	st.Add(metrics.Sample{
		TimeSeries: series,
		Value:      1.52,
		Time:       time.Unix(1, 0),
	})
	ts := st.MapPrompb(series, time.Unix(2, 0))
	require.Len(t, ts, 1)
	assert.Equal(t, "k6_test_seconds", ts[0].Labels[0].Value)
}

func TestBaseUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in  metrics.ValueType
		exp string
	}{
		{in: metrics.Default, exp: ""},
		{in: metrics.Time, exp: "seconds"},
		{in: metrics.Data, exp: "bytes"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.exp, baseUnit(tt.in))
	}
}

func BenchmarkHistogramSinkAdd(b *testing.B) {
	m := &metrics.Metric{
		Name:     "bench",
		Type:     metrics.Trend,
		Contains: metrics.Time,
	}
	ts := newNativeHistogramSink(m)
	s := metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: m,
		},
		Value: rand.Float64(), //nolint:gosec
		Time:  time.Now(),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts.Add(s)
	}
}
