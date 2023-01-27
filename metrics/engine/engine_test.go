package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/metrics"
)

func TestNewMetricsEngineWithThresholds(t *testing.T) {
	t.Parallel()

	trs := &lib.TestRunState{
		TestPreInitState: &lib.TestPreInitState{
			Logger:   testutils.NewLogger(t),
			Registry: metrics.NewRegistry(),
		},
		Options: lib.Options{
			Thresholds: map[string]metrics.Thresholds{
				"metric1": {Thresholds: []*metrics.Threshold{}},
				"metric2": {Thresholds: []*metrics.Threshold{
					{
						Source: "count>1",
					},
				}},
			},
		},
	}
	_, err := trs.Registry.NewMetric("metric1", metrics.Counter)
	require.NoError(t, err)

	_, err = trs.Registry.NewMetric("metric2", metrics.Counter)
	require.NoError(t, err)

	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)

	es := lib.NewExecutionState(trs, et, 0, 0)
	me, err := NewMetricsEngine(es)
	require.NoError(t, err)
	require.NotNil(t, me)

	assert.Len(t, me.metricsWithThresholds, 1)
}

func TestMetricsEngineGetThresholdMetricOrSubmetricError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		metricDefinition string
		expErr           string
	}{
		{metricDefinition: "metric1{test:a", expErr: "missing ending bracket"},
		{metricDefinition: "metric2", expErr: "'metric2' does not exist in the script"},
		{metricDefinition: "metric1{}", expErr: "submetric criteria for metric 'metric1' cannot be empty"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()

			me := newTestMetricsEngine(t)
			_, err := me.test.Registry.NewMetric("metric1", metrics.Counter)
			require.NoError(t, err)

			_, err = me.getThresholdMetricOrSubmetric(tc.metricDefinition)
			assert.ErrorContains(t, err, tc.expErr)
		})
	}
}

func TestNewMetricsEngineNoThresholds(t *testing.T) {
	t.Parallel()

	trs := &lib.TestRunState{
		TestPreInitState: &lib.TestPreInitState{
			Logger: testutils.NewLogger(t),
		},
	}
	et, err := lib.NewExecutionTuple(nil, nil)
	require.NoError(t, err)

	es := lib.NewExecutionState(trs, et, 0, 0)
	me, err := NewMetricsEngine(es)
	require.NoError(t, err)
	require.NotNil(t, me)

	assert.Empty(t, me.metricsWithThresholds)
}

func TestMetricsEngineCreateIngester(t *testing.T) {
	t.Parallel()

	me := MetricsEngine{
		logger: testutils.NewLogger(t),
	}
	ingester := me.CreateIngester()
	assert.NotNil(t, ingester)
	require.NoError(t, ingester.Start())
	require.NoError(t, ingester.Stop())
}

func TestMetricsEngineEvaluateThresholdNoAbort(t *testing.T) {
	t.Parallel()

	cases := []struct {
		threshold   string
		abortOnFail bool
		expBreached []string
	}{
		{threshold: "count>5", expBreached: nil},
		{threshold: "count<5", expBreached: []string{"m1"}},
		{threshold: "count<5", expBreached: []string{"m1"}, abortOnFail: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.threshold, func(t *testing.T) {
			t.Parallel()
			me := newTestMetricsEngine(t)

			m1, err := me.test.Registry.NewMetric("m1", metrics.Counter)
			require.NoError(t, err)
			m2, err := me.test.Registry.NewMetric("m2", metrics.Counter)
			require.NoError(t, err)

			ths := metrics.NewThresholds([]string{tc.threshold})
			require.NoError(t, ths.Parse())
			// m1.Thresholds = ths
			// m1.Thresholds.Thresholds[0].AbortOnFail = tc.abortOnFail
			ths.Thresholds[0].AbortOnFail = tc.abortOnFail

			// me.metricsWithThresholds = []*metrics.Metric{m1, m2}
			me.metricsWithThresholds[m1] = ths
			me.metricsWithThresholds[m2] = metrics.Thresholds{}
			m1.Sink.Add(metrics.Sample{Value: 6.0})

			breached, abort := me.evaluateThresholds(false)
			require.Equal(t, tc.abortOnFail, abort)
			assert.Equal(t, tc.expBreached, breached)
		})
	}
}

func TestMetricsEngineEvaluateIgnoreEmptySink(t *testing.T) {
	t.Parallel()

	me := newTestMetricsEngine(t)

	m1, err := me.test.Registry.NewMetric("m1", metrics.Counter)
	require.NoError(t, err)
	m2, err := me.test.Registry.NewMetric("m2", metrics.Counter)
	require.NoError(t, err)

	ths := metrics.NewThresholds([]string{"count>5"})
	require.NoError(t, ths.Parse())
	// m1.Thresholds = ths
	// m1.Thresholds.Thresholds[0].AbortOnFail = true
	ths.Thresholds[0].AbortOnFail = true

	// me.metricsWithThresholds = []*metrics.Metric{m1, m2}
	me.metricsWithThresholds[m1] = ths
	me.metricsWithThresholds[m2] = metrics.Thresholds{}

	breached, abort := me.evaluateThresholds(false)
	require.True(t, abort)
	require.Equal(t, []string{"m1"}, breached)

	breached, abort = me.evaluateThresholds(true)
	require.False(t, abort)
	assert.Empty(t, breached)
}

func TestMetricsEngineDetectedThresholds(t *testing.T) {
	t.Parallel()

	me := newTestMetricsEngine(t)

	m1, err := me.test.Registry.NewMetric("m1", metrics.Counter)
	require.NoError(t, err)
	m2, err := me.test.Registry.NewMetric("m2", metrics.Counter)
	require.NoError(t, err)

	ths := metrics.NewThresholds([]string{"count>5"})
	require.NoError(t, ths.Parse())
	ths.Thresholds[0].AbortOnFail = true

	me.metricsWithThresholds[m1] = ths
	me.metricsWithThresholds[m2] = metrics.Thresholds{}

	mwths := me.DetectedThresholds()
	require.Len(t, mwths, 2)

	assert.Equal(t, mwths[m1], ths)
	assert.NotNil(t, mwths[m2], ths)
}

func newTestMetricsEngine(t *testing.T) MetricsEngine {
	trs := &lib.TestRunState{
		TestPreInitState: &lib.TestPreInitState{
			Logger:   testutils.NewLogger(t),
			Registry: metrics.NewRegistry(),
		},
	}

	return MetricsEngine{
		logger: trs.Logger,
		test: &testRunState{
			TestRunState: trs,
			testDuration: func() time.Duration {
				return 0
			},
		},
		metricsWithThresholds: make(map[*metrics.Metric]metrics.Thresholds),
	}
}
