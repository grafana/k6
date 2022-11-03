package remotewrite

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	prompb "go.buf.build/grpc/go/prometheus/prometheus"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

func TestOutputDescription(t *testing.T) {
	t.Parallel()
	o := Output{
		config: Config{
			URL: null.StringFrom("http://remote-url.fake"),
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
	}

	o := Output{
		tsdb: make(map[metrics.TimeSeries]*seriesWithMeasure),
	}

	pbseries := o.convertToPbSeries(samples)
	require.Len(t, pbseries, 2)
	require.Len(t, o.tsdb, 2)

	unix1sept := int64(1661990400 * 1000) // in ms
	exp := []*prompb.TimeSeries{
		{
			Labels: []*prompb.Label{
				{Name: "__name__", Value: "k6_metric1"},
				{Name: "tagk1", Value: "tagv1"},
			},
			Samples: []*prompb.Sample{
				{Value: 7, Timestamp: unix1sept},
			},
		},
		{
			Labels: []*prompb.Label{
				{Name: "__name__", Value: "k6_metric2"},
				{Name: "tagk1", Value: "tagv1"},
			},
			Samples: []*prompb.Sample{
				{Value: 2, Timestamp: unix1sept},
			},
		},
	}

	assert.Equal(t, exp, pbseries)
}

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

		swm := newSeriesWithMeasure(s, false)
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

	swm := newSeriesWithMeasure(s, true)
	require.NotNil(t, swm)
	assert.Equal(t, s, swm.TimeSeries)
	require.NotNil(t, swm.Measure)

	nhs, ok := swm.Measure.(*nativeHistogramSink)
	require.True(t, ok)
	assert.NotNil(t, nhs.H)
}
