package engine

import (
	"strconv"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func TestIngesterOutputFlushMetrics(t *testing.T) {
	t.Parallel()

	piState := newTestPreInitState(t)
	testMetric, err := piState.Registry.NewMetric("test_metric", metrics.Trend)
	require.NoError(t, err)

	ingester := OutputIngester{
		logger: piState.Logger,
		metricsEngine: &MetricsEngine{
			ObservedMetrics: make(map[string]*metrics.Metric),
		},
		cardinality: newCardinalityControl(),
	}
	require.NoError(t, ingester.Start())
	ingester.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		TimeSeries: metrics.TimeSeries{Metric: testMetric},
		Value:      21,
	}})
	ingester.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		TimeSeries: metrics.TimeSeries{Metric: testMetric},
		Value:      21,
	}})
	require.NoError(t, ingester.Stop())

	require.Len(t, ingester.metricsEngine.ObservedMetrics, 1)
	metric := ingester.metricsEngine.ObservedMetrics["test_metric"]
	require.NotNil(t, metric)
	require.NotNil(t, metric.Sink)
	assert.Equal(t, testMetric, metric)

	sink := metric.Sink.(*metrics.TrendSink)
	assert.Equal(t, 42.0, sink.Total())
}

func TestIngesterOutputFlushSubmetrics(t *testing.T) {
	t.Parallel()

	piState := newTestPreInitState(t)
	testMetric, err := piState.Registry.NewMetric("test_metric", metrics.Gauge)
	require.NoError(t, err)

	me := &MetricsEngine{
		logger:          piState.Logger,
		registry:        piState.Registry,
		ObservedMetrics: make(map[string]*metrics.Metric),
	}
	_, err = me.getThresholdMetricOrSubmetric("test_metric{a:1}")
	require.NoError(t, err)

	// assert that observed metrics is empty before to start
	require.Empty(t, me.ObservedMetrics)

	ingester := OutputIngester{
		logger:        piState.Logger,
		metricsEngine: me,
		cardinality:   newCardinalityControl(),
	}
	require.NoError(t, ingester.Start())
	ingester.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: testMetric,
			Tags: piState.Registry.RootTagSet().WithTagsFromMap(
				map[string]string{"a": "1", "b": "2"}),
		},
		Value: 21,
	}})
	require.NoError(t, ingester.Stop())

	require.Len(t, ingester.metricsEngine.ObservedMetrics, 2)

	// assert the parent has been observed
	metric := ingester.metricsEngine.ObservedMetrics["test_metric"]
	require.NotNil(t, metric)
	require.NotNil(t, metric.Sink)
	assert.IsType(t, &metrics.GaugeSink{}, metric.Sink)

	// assert the submetric has been observed
	metric = ingester.metricsEngine.ObservedMetrics["test_metric{a:1}"]
	require.NotNil(t, metric)
	require.NotNil(t, metric.Sink)
	require.NotNil(t, metric.Sub)
	assert.EqualValues(t, map[string]string{"a": "1"}, metric.Sub.Tags.Map())
	assert.IsType(t, &metrics.GaugeSink{}, metric.Sink)
}

func TestOutputFlushMetricsTimeSeriesWarning(t *testing.T) {
	t.Parallel()

	piState := newTestPreInitState(t)
	testMetric, err := piState.Registry.NewMetric("test_metric", metrics.Gauge)
	require.NoError(t, err)

	logger, hook := testutils.NewLoggerWithHook(nil)
	ingester := OutputIngester{
		logger: logger,
		metricsEngine: &MetricsEngine{
			ObservedMetrics: make(map[string]*metrics.Metric),
		},
		cardinality: newCardinalityControl(),
	}
	ingester.cardinality.timeSeriesLimit = 2 // mock the limit

	require.NoError(t, ingester.Start())
	for i := 0; i < 3; i++ {
		ingester.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: testMetric,
				Tags: piState.Registry.RootTagSet().WithTagsFromMap(
					map[string]string{"a": "1", "b": strconv.Itoa(i)}),
			},
			Value: 21,
		}})
	}
	require.NoError(t, ingester.Stop())

	// to keep things simple the internal limit is not passed to the message
	// the code uses directly the global constant limit
	expLine := "generated metrics with 3 unique time series, " +
		"which is higher than the suggested limit of 100000"
	assert.True(t, testutils.LogContains(hook.Drain(), logrus.WarnLevel, expLine))
}

func TestCardinalityControlAdd(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	m1, err := registry.NewMetric("metric1", metrics.Counter)
	require.NoError(t, err)

	m2, err := registry.NewMetric("metric2", metrics.Counter)
	require.NoError(t, err)

	tags := registry.RootTagSet().With("k", "v")

	cc := newCardinalityControl()
	// the first iteration adds two new time series
	// the second does not change the count
	// because the time series have been already seen before
	for i := 0; i < 2; i++ {
		cc.Add(metrics.TimeSeries{
			Metric: m1,
			Tags:   tags,
		})
		cc.Add(metrics.TimeSeries{
			Metric: m2,
			Tags:   tags,
		})
		assert.Equal(t, 2, len(cc.seen))
	}
}

func TestCardinalityControlLimitHit(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	m1, err := registry.NewMetric("metric1", metrics.Counter)
	require.NoError(t, err)

	cc := newCardinalityControl()
	cc.timeSeriesLimit = 1

	cc.Add(metrics.TimeSeries{
		Metric: m1,
		Tags:   registry.RootTagSet().With("k", "1"),
	})
	assert.False(t, cc.LimitHit())

	// the same time series should not impact the counter
	cc.Add(metrics.TimeSeries{
		Metric: m1,
		Tags:   registry.RootTagSet().With("k", "1"),
	})
	assert.False(t, cc.LimitHit())

	cc.Add(metrics.TimeSeries{
		Metric: m1,
		Tags:   registry.RootTagSet().With("k", "2"),
	})
	assert.True(t, cc.LimitHit())
	assert.Equal(t, 2, cc.timeSeriesLimit, "the limit is expected to be raised")
}

func newTestPreInitState(tb testing.TB) *lib.TestPreInitState {
	reg := metrics.NewRegistry()
	logger := testutils.NewLogger(tb)
	return &lib.TestPreInitState{
		Logger:         logger,
		RuntimeOptions: lib.RuntimeOptions{},
		Registry:       reg,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(reg),
	}
}
