// Package engine contains the internal metrics engine responsible for
// aggregating metrics during the test and evaluating thresholds against them.
package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
	"gopkg.in/guregu/null.v3"
)

const thresholdsRate = 2 * time.Second

type TrackedMetric struct {
	*metrics.Metric

	sink     metrics.Sink
	observed bool
	m        sync.Mutex

	// TODO: store thresholds
	// thresholds metrics.Thresholds
}

func (om *TrackedMetric) AddSamples(samples ...metrics.Sample) {
	om.m.Lock()
	defer om.m.Unlock()

	for _, s := range samples {
		om.sink.Add(s)
	}

	if !om.observed {
		om.observed = true
	}

	// TODO: run the thresholds
}

// MetricsEngine is the internal metrics engine that k6 uses to keep track of
// aggregated metric sample values. They are used to generate the end-of-test
// summary and to evaluate the test thresholds.
type MetricsEngine struct {
	logger         logrus.FieldLogger
	test           *lib.TestRunState
	outputIngester *outputIngester

	// they can be both top-level metrics or sub-metrics
	//
	// TODO: move thresholds as observed metrics and run
	// the check directly when a sample is added.
	//
	// TODO: remove the tracked map using the sequence number
	metricsWithThresholds map[*metrics.Metric]metrics.Thresholds
	trackedMetrics        map[*metrics.Metric]*TrackedMetric

	breachedThresholdsCount uint32
}

// NewMetricsEngine creates a new metrics Engine with the given parameters.
func NewMetricsEngine(runState *lib.TestRunState) (*MetricsEngine, error) {
	me := &MetricsEngine{
		test:                  runState,
		logger:                runState.Logger.WithField("component", "metrics-engine"),
		metricsWithThresholds: make(map[*metrics.Metric]metrics.Thresholds),
		trackedMetrics:        make(map[*metrics.Metric]*TrackedMetric),
	}

	for _, registered := range me.test.Registry.All() {
		typ := registered.Type
		me.trackedMetrics[registered] = &TrackedMetric{
			Metric: registered,
			sink:   newSinkByType(typ),
		}
	}

	if !(me.test.RuntimeOptions.NoSummary.Bool && me.test.RuntimeOptions.NoThresholds.Bool) {
		err := me.initSubMetricsAndThresholds()
		if err != nil {
			return nil, err
		}
	}

	return me, nil
}

// CreateIngester returns a pseudo-Output that uses the given metric samples to
// update the engine's inner state.
func (me *MetricsEngine) CreateIngester() output.Output {
	me.outputIngester = &outputIngester{
		logger:        me.logger.WithField("component", "metrics-engine-ingester"),
		metricsEngine: me,
	}
	return me.outputIngester
}

func (me *MetricsEngine) getThresholdMetricOrSubmetric(name string) (*metrics.Metric, error) {
	// TODO: replace with strings.Cut after Go 1.18
	nameParts := strings.SplitN(name, "{", 2)

	metric := me.test.Registry.Get(nameParts[0])
	if metric == nil {
		return nil, fmt.Errorf("metric '%s' does not exist in the script", nameParts[0])
	}

	if len(nameParts) == 1 { // no sub-metric
		return metric, nil
	}

	submetricDefinition := nameParts[1]
	if submetricDefinition[len(submetricDefinition)-1] != '}' {
		return nil, fmt.Errorf("missing ending bracket, sub-metric format needs to be 'metric{key:value}'")
	}
	sm, err := metric.AddSubmetric(submetricDefinition[:len(submetricDefinition)-1])
	if err != nil {
		return nil, err
	}

	if _, ok := sm.Tags.Get("vu"); ok {
		me.logger.Warnf(
			"The high-cardinality 'vu' metric tag was made non-indexable in k6 v0.41.0, so thresholds"+
				" like '%s' that are based on it won't work correctly.",
			name,
		)
	}

	if _, ok := sm.Tags.Get("iter"); ok {
		me.logger.Warnf(
			"The high-cardinality 'iter' metric tag was made non-indexable in k6 v0.41.0, so thresholds"+
				" like '%s' that are based on it won't work correctly.",
			name,
		)
	}

	return sm.Metric, nil
}

func (me *MetricsEngine) initSubMetricsAndThresholds() error {
	for metricName, thresholds := range me.test.Options.Thresholds {
		metric, err := me.getThresholdMetricOrSubmetric(metricName)

		if me.test.RuntimeOptions.NoThresholds.Bool {
			if err != nil {
				me.logger.WithError(err).Warnf("Invalid metric '%s' in threshold definitions", metricName)
			}
			continue
		}

		if err != nil {
			return fmt.Errorf("invalid metric '%s' in threshold definitions: %w", metricName, err)
		}

		// TODO: check and confirm that this check is not an issue
		if len(thresholds.Thresholds) > 0 {
			me.metricsWithThresholds[metric] = thresholds
		}

		// Mark the metric (and the parent metric, if we're dealing with a
		// submetric) as observed, so they are shown in the end-of-test summary,
		// even if they don't have any metric samples during the test run

		me.trackedMetrics[metric] = &TrackedMetric{
			Metric:   metric,
			sink:     newSinkByType(metric.Type),
			observed: true,
			// thresholds: thresholds,
		}

		if metric.Sub != nil {
			me.trackedMetrics[metric.Sub.Parent] = &TrackedMetric{
				Metric:   metric.Sub.Parent,
				sink:     newSinkByType(metric.Sub.Parent.Type),
				observed: true,
			}
		}
	}

	// TODO: refactor out of here when https://github.com/grafana/k6/issues/1321
	// lands and there is a better way to enable a metric with tag
	if me.test.Options.SystemTags.Has(metrics.TagExpectedResponse) {
		expResMetric, err := me.getThresholdMetricOrSubmetric("http_req_duration{expected_response:true}")
		if err != nil {
			return err // shouldn't happen, but ¯\_(ツ)_/¯
		}
		me.trackedMetrics[expResMetric] = &TrackedMetric{
			Metric: expResMetric,
			sink:   newSinkByType(expResMetric.Type),
		}
	}

	return nil
}

// StartThresholdCalculations spins up a new goroutine to crunch thresholds and
// returns a callback that will stop the goroutine and finalizes calculations.
func (me *MetricsEngine) StartThresholdCalculations(
	abortRun func(error),
	getCurrentTestRunDuration func() time.Duration,
) (finalize func() (breached []string),
) {
	if len(me.metricsWithThresholds) == 0 {
		return nil // no thresholds were defined
	}

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(thresholdsRate)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				breached, shouldAbort := me.evaluateThresholds(true, getCurrentTestRunDuration)
				if shouldAbort {
					err := fmt.Errorf(
						"thresholds on metrics '%s' were breached; at least one has abortOnFail enabled, stopping test prematurely",
						strings.Join(breached, ", "),
					)
					me.logger.Debug(err.Error())
					err = errext.WithAbortReasonIfNone(
						errext.WithExitCodeIfNone(err, exitcodes.ThresholdsHaveFailed), errext.AbortedByThreshold,
					)
					abortRun(err)
				}
			case <-stop:
				return
			}
		}
	}()

	return func() []string {
		if me.outputIngester != nil {
			// Stop the ingester so we don't get any more metrics
			err := me.outputIngester.Stop()
			if err != nil {
				me.logger.WithError(err).Warnf("There was a problem stopping the output ingester.")
			}
		}
		close(stop)
		<-done

		breached, _ := me.evaluateThresholds(false, getCurrentTestRunDuration)
		return breached
	}
}

// evaluateThresholds processes all of the thresholds.
//
// TODO: refactor, optimize
func (me *MetricsEngine) evaluateThresholds(
	ignoreEmptySinks bool,
	getCurrentTestRunDuration func() time.Duration,
) (breachedThresholds []string, shouldAbort bool) {
	t := getCurrentTestRunDuration()

	computeThresholds := func(m *metrics.Metric, ths metrics.Thresholds) {
		observedMetric, ok := me.trackedMetrics[m]
		if !ok {
			panic(fmt.Sprintf("observed metric %q not found for the threhsolds", m.Name))
		}

		observedMetric.m.Lock()
		defer observedMetric.m.Unlock()

		// If either the metric has no thresholds defined, or its sinks
		// are empty, let's ignore its thresholds execution at this point.
		if len(ths.Thresholds) == 0 || (ignoreEmptySinks && observedMetric.sink.IsEmpty()) {
			return
		}
		m.Tainted = null.BoolFrom(false)

		succ, err := ths.Run(observedMetric.sink, t)
		if err != nil {
			me.logger.WithField("metric_name", m.Name).WithError(err).Error("Threshold error")
			return
		}
		if succ {
			return // threshold passed
		}
		breachedThresholds = append(breachedThresholds, m.Name)
		m.Tainted = null.BoolFrom(true)
		if ths.Abort {
			shouldAbort = true
		}
	}

	me.logger.Debugf("Running thresholds on %d metrics...", len(me.metricsWithThresholds))
	for m, ths := range me.metricsWithThresholds {
		computeThresholds(m, ths)
	}

	if len(breachedThresholds) > 0 {
		sort.Strings(breachedThresholds)
		me.logger.Debugf("Thresholds on %d metrics breached: %v", len(breachedThresholds), breachedThresholds)
	}
	atomic.StoreUint32(&me.breachedThresholdsCount, uint32(len(breachedThresholds)))
	return breachedThresholds, shouldAbort
}

// DetectedThresholds ... TODO
func (me *MetricsEngine) DetectedThresholds() map[*metrics.Metric]metrics.Thresholds {
	// TODO: make a copy to be safe

	return me.metricsWithThresholds
}

func (me *MetricsEngine) Sinks() map[*metrics.Metric]metrics.Sink {
	ometrics := make(map[*metrics.Metric]metrics.Sink, len(me.trackedMetrics))
	for _, om := range me.trackedMetrics {
		if !om.observed {
			continue
		}
		ometrics[om.Metric] = om.sink
	}
	return ometrics
}

func (me *MetricsEngine) ObservedMetrics() map[string]*metrics.Metric {
	ometrics := make(map[string]*metrics.Metric, len(me.trackedMetrics))
	for _, om := range me.trackedMetrics {
		if !om.observed {
			continue
		}
		ometrics[om.Name] = om.Metric
	}
	return ometrics
}

// TODO: check and confirm this is not an issue done in this way
// it should serve an endpoint used with a low frequency
func (me *MetricsEngine) ObservedMetricByID(id string) (*metrics.Metric, bool) {
	for _, om := range me.trackedMetrics {
		if om.Name != id {
			continue
		}
		return om.Metric, true
	}
	return nil, false
}

// GetMetricsWithBreachedThresholdsCount returns the number of metrics for which
// the thresholds were breached (failed) during the last processing phase. This
// API is safe to use concurrently.
func (me *MetricsEngine) GetMetricsWithBreachedThresholdsCount() uint32 {
	return atomic.LoadUint32(&me.breachedThresholdsCount)
}

func newSinkByType(mt metrics.MetricType) metrics.Sink {
	var sink metrics.Sink
	switch mt {
	case metrics.Counter:
		sink = &metrics.CounterSink{}
	case metrics.Gauge:
		sink = &metrics.GaugeSink{}
	case metrics.Trend:
		sink = &metrics.TrendSink{}
	case metrics.Rate:
		sink = &metrics.RateSink{}
	}
	return sink
}
