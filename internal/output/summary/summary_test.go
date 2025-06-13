package summary

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/lib/consts"
	"go.k6.io/k6/internal/lib/summary"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

func TestOutput_Summary(t *testing.T) {
	t.Parallel()

	o, err := New(output.Params{
		Logger: testutils.NewLogger(t),
	})
	require.NoError(t, err)

	// Metrics
	checksMetric := &metrics.Metric{
		Name:     "checks",
		Type:     metrics.Rate,
		Contains: metrics.Default,
		Sink: &metrics.RateSink{
			Trues: 3,
			Total: 5,
		},
		Observed: true,
	}

	httpReqsMetric := &metrics.Metric{
		Name:     "http_reqs",
		Type:     metrics.Counter,
		Contains: metrics.Default,
		Sink: &metrics.CounterSink{
			Value: 4,
			First: time.Now(),
		},
		Observed: true,
	}

	authHTTPReqsMetric := &metrics.Metric{
		Name:     "http_reqs{group: ::auth}",
		Type:     metrics.Counter,
		Contains: metrics.Default,
		Sink: &metrics.CounterSink{
			Value: 1,
			First: time.Now(),
		},
		Observed: true,
	}

	// Thresholds
	thresholds := thresholds{
		httpReqsMetric.Name: {
			aggregatedMetric: relayAggregatedMetricFrom(httpReqsMetric),
			tt: []*metrics.Threshold{
				{
					Source: "count<10",
				},
				{
					Source:     "rate>2",
					LastFailed: true,
				},
			},
		},
		authHTTPReqsMetric.Name: {
			aggregatedMetric: relayAggregatedMetricFrom(authHTTPReqsMetric),
			tt: []*metrics.Threshold{
				{
					Source:     "count>1",
					LastFailed: true,
				},
			},
		},
	}

	// Checks
	quickPizzaIsUp := &summary.Check{
		Name:   "quickpizza.grafana.com is up",
		Passes: 3,
		Fails:  2,
	}

	checks := &aggregatedChecksData{
		checks:        map[string]*summary.Check{quickPizzaIsUp.Name: quickPizzaIsUp},
		orderedChecks: []*summary.Check{quickPizzaIsUp},
	}

	// Set up
	o.dataModel = dataModel{
		thresholds: thresholds,
		aggregatedGroupData: &aggregatedGroupData{
			checks: checks,
			aggregatedMetrics: map[string]aggregatedMetric{
				checksMetric.Name: {
					MetricInfo: summaryMetricInfoFrom(checksMetric),
					Sink:       checksMetric.Sink,
				},
				httpReqsMetric.Name: {
					MetricInfo: summaryMetricInfoFrom(httpReqsMetric),
					Sink:       httpReqsMetric.Sink,
				},
				authHTTPReqsMetric.Name: {
					MetricInfo: summaryMetricInfoFrom(authHTTPReqsMetric),
					Sink:       authHTTPReqsMetric.Sink,
				},
			},
			groupsData: make(map[string]*aggregatedGroupData),
		},
	}

	testRunDuration := time.Second
	observedMetrics := map[string]*metrics.Metric{
		httpReqsMetric.Name:     httpReqsMetric,
		authHTTPReqsMetric.Name: authHTTPReqsMetric,
	}
	options := lib.Options{
		SummaryTrendStats: []string{"avg", "min", "max"},
	}

	s := o.Summary(testRunDuration, observedMetrics, options)

	// Assert thresholds
	assert.Len(t, s.Thresholds, 2)

	httpReqsThresholds := s.Thresholds[httpReqsMetric.Name].Thresholds
	assert.Len(t, httpReqsThresholds, 2)
	assert.Equal(t, "count<10", httpReqsThresholds[0].Source)
	assert.True(t, httpReqsThresholds[0].Ok)
	assert.Equal(t, "rate>2", httpReqsThresholds[1].Source)
	assert.False(t, httpReqsThresholds[1].Ok)

	httpReqsGroupThresholds := s.Thresholds[authHTTPReqsMetric.Name].Thresholds
	assert.Len(t, httpReqsGroupThresholds, 1)
	assert.Equal(t, "count>1", httpReqsGroupThresholds[0].Source)
	assert.False(t, httpReqsGroupThresholds[0].Ok)

	// Assert checks
	checksTotal := s.Checks.Metrics.Total
	assert.Equal(t, "checks_total", checksTotal.Name)
	assert.Equal(t, map[string]float64{
		"count": 5,
		"rate":  5,
	}, checksTotal.Values)

	checksSucceeded := s.Checks.Metrics.Success
	assert.Equal(t, "checks_succeeded", checksSucceeded.Name)
	assert.Equal(t, map[string]float64{
		"rate":   0.6,
		"passes": 3,
		"fails":  2,
	}, checksSucceeded.Values)

	checksFailed := s.Checks.Metrics.Fail
	assert.Equal(t, "checks_failed", checksFailed.Name)
	assert.Equal(t, map[string]float64{
		"rate":   0.4,
		"passes": 2,
		"fails":  3,
	}, checksFailed.Values)

	assert.Len(t, s.Checks.OrderedChecks, 1)
	assert.Equal(t, quickPizzaIsUp, s.Checks.OrderedChecks[0])

	// Assert metrics
	assert.Len(t, s.Metrics.HTTP, 2)

	httpReqsSummaryMetric := s.Metrics.HTTP[httpReqsMetric.Name]
	assert.Equal(t, "http_reqs", httpReqsSummaryMetric.Name)
	assert.Equal(t, "counter", httpReqsSummaryMetric.Type)
	assert.Equal(t, "default", httpReqsSummaryMetric.Contains)
	assert.Equal(t, map[string]float64{
		"count": 4,
		"rate":  4,
	}, httpReqsSummaryMetric.Values)

	authHTTPReqsSummaryMetric := s.Metrics.HTTP[authHTTPReqsMetric.Name]
	assert.Equal(t, "http_reqs{group: ::auth}", authHTTPReqsSummaryMetric.Name)
	assert.Equal(t, "counter", authHTTPReqsSummaryMetric.Type)
	assert.Equal(t, "default", authHTTPReqsSummaryMetric.Contains)
	assert.Equal(t, map[string]float64{
		"count": 1,
		"rate":  1,
	}, authHTTPReqsSummaryMetric.Values)

	// Other asserts
	assert.Equal(t, testRunDuration, s.TestRunDuration)
}

func TestOutput_AddMetricSamples(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()

	httpReqsMetric := &metrics.Metric{
		Name:     "http_reqs",
		Type:     metrics.Counter,
		Contains: metrics.Default,
		Sink: &metrics.CounterSink{
			Value: 4,
			First: time.Now(),
		},
		Observed: true,
	}

	authHTTPReqsMetric := &metrics.Metric{
		Name:     "http_reqs{group: ::auth}",
		Type:     metrics.Counter,
		Contains: metrics.Default,
		Sink: &metrics.CounterSink{
			Value: 1,
			First: time.Now(),
		},
		Observed: true,
	}

	samples := []metrics.SampleContainer{
		metrics.Samples{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: httpReqsMetric,
					Tags:   reg.RootTagSet().With("group", lib.RootGroupPath),
				},
				Time:  time.Now(),
				Value: 1,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: httpReqsMetric,
					Tags:   reg.RootTagSet().With("group", lib.GroupSeparator+consts.SetupFn),
				},
				Time:  time.Now(),
				Value: 1,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: httpReqsMetric,
					Tags:   reg.RootTagSet().With("group", lib.GroupSeparator+consts.TeardownFn),
				},
				Time:  time.Now(),
				Value: 1,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: authHTTPReqsMetric,
					Tags:   reg.RootTagSet().With("group", lib.GroupSeparator+"something"),
				},
				Time:  time.Now(),
				Value: 1,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: authHTTPReqsMetric,
					Tags:   reg.RootTagSet().With("group", lib.GroupSeparator+"auth"),
				},
				Time:  time.Now(),
				Value: 1,
			},
		},
		metrics.Samples{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: httpReqsMetric,
					Tags:   reg.RootTagSet().With("group", lib.RootGroupPath),
				},
				Time:  time.Now(),
				Value: 3,
			},
		},
	}

	t.Run("compact", func(t *testing.T) {
		t.Parallel()

		o, err := New(output.Params{
			RuntimeOptions: lib.RuntimeOptions{
				SummaryMode: null.StringFrom("compact"),
			},
			Logger: testutils.NewLogger(t),
		})
		require.NoError(t, err)

		require.NoError(t, o.Start())

		o.AddMetricSamples(samples)

		require.NoError(t, o.Stop())

		assert.Len(t, o.dataModel.aggregatedMetrics, 2)

		httpReqsSummaryMetric := o.dataModel.aggregatedMetrics[httpReqsMetric.Name]
		assert.Equal(t, float64(4), httpReqsSummaryMetric.Sink.(*metrics.CounterSink).Value)

		authHTTPReqsSummaryMetric := o.dataModel.aggregatedMetrics[authHTTPReqsMetric.Name]
		assert.Equal(t, float64(1), authHTTPReqsSummaryMetric.Sink.(*metrics.CounterSink).Value)

		assert.Len(t, o.dataModel.groupsData, 0)
	})

	t.Run("full", func(t *testing.T) {
		t.Parallel()

		o, err := New(output.Params{
			RuntimeOptions: lib.RuntimeOptions{
				SummaryMode: null.StringFrom("full"),
			},
			Logger: testutils.NewLogger(t),
		})
		require.NoError(t, err)

		require.NoError(t, o.Start())

		o.AddMetricSamples(samples)

		require.NoError(t, o.Stop())

		assert.Len(t, o.dataModel.aggregatedMetrics, 2)

		httpReqsSummaryMetric := o.dataModel.aggregatedMetrics[httpReqsMetric.Name]
		assert.Equal(t, float64(4), httpReqsSummaryMetric.Sink.(*metrics.CounterSink).Value)

		authHTTPReqsSummaryMetric := o.dataModel.aggregatedMetrics[authHTTPReqsMetric.Name]
		assert.Equal(t, float64(1), authHTTPReqsSummaryMetric.Sink.(*metrics.CounterSink).Value)

		assert.Len(t, o.dataModel.groupsData, 2)
		assert.Len(t, o.dataModel.groupsData["something"].aggregatedMetrics, 1)
		assert.Len(t, o.dataModel.groupsData["auth"].aggregatedMetrics, 1)

		authHTTPReqsSummaryMetric = o.dataModel.groupsData["auth"].aggregatedMetrics[authHTTPReqsMetric.Name]
		assert.Equal(t, float64(1), authHTTPReqsSummaryMetric.Sink.(*metrics.CounterSink).Value)
		authHTTPReqsSummaryMetric = o.dataModel.groupsData["something"].aggregatedMetrics[authHTTPReqsMetric.Name]
		assert.Equal(t, float64(1), authHTTPReqsSummaryMetric.Sink.(*metrics.CounterSink).Value)

		assert.Equal(t, []string{"something", "auth"}, o.dataModel.groupsOrder)
	})
}
