package engine

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/metrics"
)

func TestIngesterOutputFlushMetrics(t *testing.T) {
	t.Parallel()

	piState := newTestPreInitState(t)
	testMetric, err := piState.Registry.NewMetric("test_metric", metrics.Trend)
	require.NoError(t, err)

	ingester := outputIngester{
		logger: piState.Logger,
		metricsEngine: &MetricsEngine{
			ObservedMetrics: make(map[string]*metrics.Metric),
		},
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

	sink := metric.Sink.(*metrics.TrendSink) //nolint:forcetypeassert
	assert.Equal(t, 42.0, sink.Sum)
}

func TestIngesterOutputFlushSubmetrics(t *testing.T) {
	t.Parallel()

	piState := newTestPreInitState(t)
	testMetric, err := piState.Registry.NewMetric("test_metric", metrics.Gauge)
	require.NoError(t, err)

	me := &MetricsEngine{
		test: &lib.TestRunState{
			TestPreInitState: piState,
		},
		ObservedMetrics: make(map[string]*metrics.Metric),
	}
	_, err = me.getThresholdMetricOrSubmetric("test_metric{a:1}")
	require.NoError(t, err)

	// assert that observed metrics is empty before to start
	require.Empty(t, me.ObservedMetrics)

	ingester := outputIngester{
		logger:        piState.Logger,
		metricsEngine: me,
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

func newTestPreInitState(tb testing.TB) *lib.TestPreInitState {
	reg := metrics.NewRegistry()
	logger := testutils.NewLogger(tb)
	logger.SetLevel(logrus.DebugLevel)
	return &lib.TestPreInitState{
		Logger:         logger,
		RuntimeOptions: lib.RuntimeOptions{},
		Registry:       reg,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(reg),
	}
}
