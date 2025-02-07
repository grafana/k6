package remotewrite

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	prompb "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

func TestOutputDescription(t *testing.T) {
	t.Parallel()
	o := Output{
		config: Config{
			ServerURL: null.StringFrom("http://remote-url.fake"),
		},
	}
	exp := "Prometheus remote write (http://remote-url.fake)"
	assert.Equal(t, exp, o.Description())
}

func TestOutputConvertToPbSeries(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	metric1 := registry.MustNewMetric("metric1", metrics.Counter)
	tagset := registry.RootTagSet().With("tagk1", "tagv1")

	samples := []metrics.SampleContainer{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: metric1,
				Tags:   tagset,
			},
			Time:  time.Date(2022, time.September, 1, 0, 0, 0, 0, time.UTC),
			Value: 3,
		},
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: metric1,
				Tags:   tagset,
			},
			Time:  time.Date(2022, time.August, 31, 0, 0, 0, 0, time.UTC),
			Value: 4,
		},
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: registry.MustNewMetric("metric2", metrics.Counter),
				Tags:   tagset,
			},
			Time:  time.Date(2022, time.September, 1, 0, 0, 0, 0, time.UTC),
			Value: 2,
		},
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: registry.MustNewMetric("metric3", metrics.Rate),
				Tags:   tagset,
			},
			Time:  time.Date(2022, time.September, 1, 0, 0, 0, 0, time.UTC),
			Value: 7,
		},
	}

	o := Output{
		tsdb: make(map[metrics.TimeSeries]*seriesWithMeasure),
	}

	pbseries := o.convertToPbSeries(samples)
	require.Len(t, pbseries, 3)
	require.Len(t, o.tsdb, 3)

	unix1sept := int64(1661990400 * 1000) // in ms
	exp := []*prompb.TimeSeries{
		{
			Labels: []*prompb.Label{
				{Name: "__name__", Value: "k6_metric1_total"},
				{Name: "tagk1", Value: "tagv1"},
			},
			Samples: []*prompb.Sample{
				{Value: 7, Timestamp: unix1sept},
			},
		},
		{
			Labels: []*prompb.Label{
				{Name: "__name__", Value: "k6_metric2_total"},
				{Name: "tagk1", Value: "tagv1"},
			},
			Samples: []*prompb.Sample{
				{Value: 2, Timestamp: unix1sept},
			},
		},
		{
			Labels: []*prompb.Label{
				{Name: "__name__", Value: "k6_metric3_rate"},
				{Name: "tagk1", Value: "tagv1"},
			},
			Samples: []*prompb.Sample{
				{Value: 1, Timestamp: unix1sept},
			},
		},
	}

	sortByNameLabel(pbseries)
	assert.Equal(t, exp, pbseries)
}

//nolint:paralleltest,tparallel
func TestOutputConvertToPbSeries_WithPreviousState(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	metric1 := registry.MustNewMetric("metric1", metrics.Counter)
	tagset := registry.RootTagSet().With("tagk1", "tagv1")
	t0 := time.Date(2022, time.September, 1, 0, 0, 0, 0, time.UTC).Add(10 * time.Millisecond)

	swm := &seriesWithMeasure{
		TimeSeries: metrics.TimeSeries{
			Metric: metric1,
			Tags:   tagset,
		},
		Latest: t0,
		// it's not relevant for this test to initialize the Sink's values
		Measure: &metrics.CounterSink{},
	}

	o := Output{
		tsdb: map[metrics.TimeSeries]*seriesWithMeasure{
			swm.TimeSeries: swm,
		},
	}

	testcases := []struct {
		name      string
		time      time.Time
		expSeries int
		expCount  float64
		expLatest time.Time
	}{
		{
			name:      "Before",
			time:      time.Date(2022, time.August, 31, 0, 0, 0, 0, time.UTC),
			expSeries: 0,
			expCount:  1,
			expLatest: t0,
		},
		{
			name:      "AfterButSub-ms", // so equal when truncated
			time:      t0.Add(10 * time.Microsecond),
			expSeries: 0,
			expCount:  2,
			expLatest: time.Date(2022, time.September, 1, 0, 0, 0, int(10*time.Millisecond), time.UTC),
		},
		{
			name:      "After",
			time:      t0.Add(1 * time.Millisecond),
			expSeries: 1,
			expCount:  3,
			expLatest: time.Date(2022, time.September, 1, 0, 0, 0, int(11*time.Millisecond), time.UTC),
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pbseries := o.convertToPbSeries([]metrics.SampleContainer{
				metrics.Sample{
					TimeSeries: metrics.TimeSeries{
						Metric: metric1,
						Tags:   tagset,
					},
					Value: 1,
					Time:  tc.time,
				},
			})
			require.Len(t, o.tsdb, 1)
			require.Equal(t, tc.expSeries, len(pbseries))
			assert.Equal(t, tc.expCount, swm.Measure.(*metrics.CounterSink).Value)
			assert.Equal(t, tc.expLatest, swm.Latest)
		})
	}
}

func TestNewSeriesWithK6SinkMeasure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expSink    metrics.Sink
		metricType metrics.MetricType
	}{
		{
			metricType: metrics.Counter,
			expSink:    &metrics.CounterSink{},
		},
		{
			metricType: metrics.Gauge,
			expSink:    &metrics.GaugeSink{},
		},
		{
			metricType: metrics.Rate,
			expSink:    &metrics.RateSink{},
		},
		{
			metricType: metrics.Trend,
			expSink:    &extendedTrendSink{},
		},
	}

	registry := metrics.NewRegistry()
	for i, tt := range tests {
		s := metrics.TimeSeries{
			Metric: registry.MustNewMetric(fmt.Sprintf("metric%d", i), tt.metricType),
		}
		resolvers, err := metrics.GetResolversForTrendColumns([]string{"avg"})
		require.NoError(t, err)
		swm := newSeriesWithMeasure(s, false, resolvers)
		require.NotNil(t, swm)
		assert.Equal(t, s, swm.TimeSeries)
		require.NotNil(t, swm.Measure)
		assert.IsType(t, tt.expSink, swm.Measure)
	}
}

func TestNewSeriesWithNativeHistogramMeasure(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	s := metrics.TimeSeries{
		Metric: registry.MustNewMetric("metric1", metrics.Trend),
	}

	swm := newSeriesWithMeasure(s, true, nil)
	require.NotNil(t, swm)
	assert.Equal(t, s, swm.TimeSeries)
	require.NotNil(t, swm.Measure)

	nhs, ok := swm.Measure.(*nativeHistogramSink)
	require.True(t, ok)
	assert.NotNil(t, nhs.H)
}

func TestOutputSetTrendStatsResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		stats           []string
		expResolverKeys []string
	}{
		{
			stats:           []string{},
			expResolverKeys: []string{},
		},
		{
			stats:           []string{"sum"},
			expResolverKeys: []string{"sum"},
		},
		{
			stats:           []string{"avg"},
			expResolverKeys: []string{"avg"},
		},
		{
			stats:           []string{"p(27)", "p(0.999)", "p(1)", "p(0)"},
			expResolverKeys: []string{"p27", "p0999", "p1", "p0"},
		},
		{
			stats: []string{
				"count", "sum",
				"max", "min", "med", "avg", "p(90)", "p(99)",
			},
			expResolverKeys: []string{
				"count", "sum",
				"max", "min", "med", "avg", "p90", "p99",
			},
		},
	}

	for _, tt := range tests {
		o := Output{}
		err := o.setTrendStatsResolver(tt.stats)
		require.NoError(t, err)
		require.NotNil(t, o.trendStatsResolver)

		assert.Len(t, o.trendStatsResolver, len(tt.expResolverKeys))
		assert.ElementsMatch(t, tt.expResolverKeys, func() []string {
			var keys []string
			for statKey := range o.trendStatsResolver {
				keys = append(keys, statKey)
			}
			return keys
		}())
	}
}

func TestOutputStaleMarkers(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	trendSinkSeries := metrics.TimeSeries{
		Metric: registry.MustNewMetric("metric1", metrics.Trend),
		Tags:   registry.RootTagSet(),
	}
	counterSinkSeries := metrics.TimeSeries{
		Metric: registry.MustNewMetric("metric2", metrics.Counter),
		Tags:   registry.RootTagSet(),
	}

	o := Output{
		now: func() time.Time {
			return time.Unix(1, 0)
		},
	}
	err := o.setTrendStatsResolver([]string{"p(99)"})
	require.NoError(t, err)
	trendSink, err := newExtendedTrendSink(o.trendStatsResolver)
	require.NoError(t, err)

	o.tsdb = map[metrics.TimeSeries]*seriesWithMeasure{
		trendSinkSeries: {
			TimeSeries: trendSinkSeries,
			// TODO: if Measure is a lighter interface
			// then it can be just a mapper mock.
			Measure: trendSink,
		},
		counterSinkSeries: {
			TimeSeries: counterSinkSeries,
			Measure:    &metrics.CounterSink{},
		},
	}

	markers := o.staleMarkers()
	require.Len(t, markers, 2)

	sortByNameLabel(markers)
	expNameLabels := []string{"k6_metric1_p99", "k6_metric2_total"}
	expTimestamp := time.Unix(1, int64(1*time.Millisecond)).UnixMilli()
	for i, expName := range expNameLabels {
		assert.Equal(t, expName, markers[i].Labels[0].Value)
		assert.Equal(t, expTimestamp, markers[i].Samples[0].Timestamp)
		assert.True(t, math.IsNaN(markers[i].Samples[0].Value), "it isn't a StaleNaN value")
	}
}

func TestOutputStopWithStaleMarkers(t *testing.T) {
	t.Parallel()

	for _, tc := range []bool{true, false} {
		buf := bytes.NewBuffer(nil)
		logger := logrus.New()
		logger.SetLevel(logrus.DebugLevel)
		logger.SetOutput(buf)

		o := Output{
			logger: logger,
			config: Config{
				// setting a large interval so it does not trigger
				// and the trigger can be inoked only when Stop is
				// invoked.
				PushInterval: types.NullDurationFrom(1 * time.Hour),
				StaleMarkers: null.BoolFrom(tc),
			},
			now: time.Now,
		}

		err := o.Start()
		require.NoError(t, err)
		err = o.Stop()
		require.NoError(t, err)

		// TODO: it isn't optimal to maintain
		// if a new logline is added in Start or flushMetrics
		// then this test will break
		// A mock of the client and check if Store is invoked
		// should be a more stable method.
		messages := buf.String()
		msg := "No time series to mark as stale"
		assertfn := assert.Contains
		if !tc {
			assertfn = assert.NotContains
		}
		assertfn(t, messages, msg)
	}
}
