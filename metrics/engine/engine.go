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

type trackedMetric struct {
	*metrics.Metric

	sink     metrics.Sink
	observed bool
	tainted  bool
	m        sync.Mutex
}

func (om *trackedMetric) AddSamples(samples ...metrics.Sample) {
	om.m.Lock()
	defer om.m.Unlock()

	for _, s := range samples {
		om.sink.Add(s)
	}

	if !om.observed {
		om.observed = true
	}
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
	// TODO: remove the tracked map using the sequence number
	metricsWithThresholds map[uint64]metrics.Thresholds
	trackedMetrics        []*trackedMetric

	breachedThresholdsCount uint32
}

// NewMetricsEngine creates a new metrics Engine with the given parameters.
func NewMetricsEngine(runState *lib.TestRunState) (*MetricsEngine, error) {
	me := &MetricsEngine{
		test:                  runState,
		logger:                runState.Logger.WithField("component", "metrics-engine"),
		metricsWithThresholds: make(map[uint64]metrics.Thresholds),
	}

	if me.test.RuntimeOptions.NoSummary.Bool &&
		me.test.RuntimeOptions.NoThresholds.Bool {
		return me, nil
	}

	// It adds all the registered metrics as tracked
	// the custom metrics are also added because they have
	// been seen and registered during the initEnv run
	// that must run before this constructor is called.
	registered := me.test.Registry.All()
	me.trackedMetrics = make([]*trackedMetric, len(registered)+1)
	for _, mreg := range registered {
		me.trackMetric(mreg)
	}

	// It adds register and tracks all the metrics defined by the thresholds.
	// They are also marked as observed because
	// the summary wants them also if they didn't receive any sample.
	err := me.initSubMetricsAndThresholds()
	if err != nil {
		return nil, err
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

		if len(thresholds.Thresholds) > 0 {
			me.metricsWithThresholds[metric.ID] = thresholds
		}

		// Mark the metric (and the parent metric, if we're dealing with a
		// submetric) as observed, so they are shown in the end-of-test summary,
		// even if they don't have any metric samples during the test run.
		me.trackMetric(metric)
		me.trackedMetrics[metric.ID].observed = true

		if metric.Sub != nil {
			me.trackMetric(metric.Sub.Parent)
			me.trackedMetrics[metric.Sub.Parent.ID].observed = true
		}
	}

	// TODO: refactor out of here when https://github.com/grafana/k6/issues/1321
	// lands and there is a better way to enable a metric with tag
	if me.test.Options.SystemTags.Has(metrics.TagExpectedResponse) {
		expResMetric, err := me.getThresholdMetricOrSubmetric("http_req_duration{expected_response:true}")
		if err != nil {
			return err // shouldn't happen, but ¯\_(ツ)_/¯
		}
		me.trackMetric(expResMetric)
	}

	// TODO: the trackedMetrics slice is fixed now
	// to be optimal we could shrink the slice cap

	return nil
}

func (me *MetricsEngine) trackMetric(m *metrics.Metric) {
	tm := &trackedMetric{
		Metric: m,
		sink:   metrics.NewSinkByType(m.Type),
	}

	if me.trackedMetrics == nil {
		// the Metric ID starts from one
		// so it skips the zero-th position
		// to simplify the access operations.
		me.trackedMetrics = []*trackedMetric{nil}
	}

	if m.ID >= uint64(len(me.trackedMetrics)) {
		// expand the slice
		me.trackedMetrics = append(me.trackedMetrics, tm)
		return
	}

	me.trackedMetrics[m.ID] = tm
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

	computeThresholds := func(tm *trackedMetric, ths metrics.Thresholds) {
		tm.m.Lock()
		defer tm.m.Unlock()

		// If either the metric has no thresholds defined, or its sinks
		// are empty, let's ignore its thresholds execution at this point.
		if len(ths.Thresholds) == 0 || (ignoreEmptySinks && tm.sink.IsEmpty()) {
			return
		}
		tm.tainted = false

		succ, err := ths.Run(tm.sink, t)
		if err != nil {
			me.logger.WithField("metric_name", tm.Name).WithError(err).Error("Threshold error")
			return
		}
		if succ {
			return // threshold passed
		}
		breachedThresholds = append(breachedThresholds, tm.Name)
		tm.tainted = true
		if ths.Abort {
			shouldAbort = true
		}
	}

	me.logger.Debugf("Running thresholds on %d metrics...", len(me.metricsWithThresholds))
	for mid, ths := range me.metricsWithThresholds {
		tracked := me.trackedMetrics[mid]
		computeThresholds(tracked, ths)
	}

	if len(breachedThresholds) > 0 {
		sort.Strings(breachedThresholds)
		me.logger.Debugf("Thresholds on %d metrics breached: %v", len(breachedThresholds), breachedThresholds)
	}
	atomic.StoreUint32(&me.breachedThresholdsCount, uint32(len(breachedThresholds)))
	return breachedThresholds, shouldAbort
}

// ObservedMetrics returns all observed metrics.
func (me *MetricsEngine) ObservedMetrics() map[string]metrics.ObservedMetric {
	ometrics := make(map[string]metrics.ObservedMetric, len(me.trackedMetrics))

	// it skips the first item as it is nil by definition
	for i := 1; i < len(me.trackedMetrics); i++ {
		tm := me.trackedMetrics[i]
		tm.m.Lock()
		if !tm.observed {
			tm.m.Unlock()
			continue
		}
		ometrics[tm.Name] = me.trackedToObserved(tm)
		tm.m.Unlock()
	}
	return ometrics
}

// ObservedMetricByID returns the observed metric by the provided id.
func (me *MetricsEngine) ObservedMetricByID(id string) (metrics.ObservedMetric, bool) {
	m := me.test.Registry.Get(id)
	if m == nil {
		return metrics.ObservedMetric{}, false
	}

	tm := me.trackedMetrics[m.ID]
	tm.m.Lock()
	defer tm.m.Unlock()

	if !tm.observed {
		return metrics.ObservedMetric{}, false
	}
	return me.trackedToObserved(tm), true
}

// trackedToObserved executes a memory safe copy to adapt from
// a dynamic tracked metric to a static observed metric.
func (me *MetricsEngine) trackedToObserved(tm *trackedMetric) metrics.ObservedMetric {
	var sink metrics.Sink
	switch sinktyp := tm.sink.(type) {
	case *metrics.CounterSink:
		sinkCopy := *sinktyp
		sink = &sinkCopy
	case *metrics.GaugeSink:
		sinkCopy := *sinktyp
		sink = &sinkCopy
	case *metrics.RateSink:
		sinkCopy := *sinktyp
		sink = &sinkCopy
	case *metrics.TrendSink:
		sinkCopy := *sinktyp
		sink = &sinkCopy
	}

	om := metrics.ObservedMetric{
		Metric:  tm.Metric,
		Sink:    sink,
		Tainted: null.BoolFrom(tm.tainted), // TODO: if null it's required then add to trackedMetric
	}

	definedThs, ok := me.metricsWithThresholds[tm.ID]
	if !ok || len(definedThs.Thresholds) < 1 {
		return om
	}

	ths := make([]metrics.Threshold, 0, len(definedThs.Thresholds))
	for _, th := range definedThs.Thresholds {
		ths = append(ths, *th)
	}

	om.Thresholds = ths
	return om
}

// GetMetricsWithBreachedThresholdsCount returns the number of metrics for which
// the thresholds were breached (failed) during the last processing phase. This
// API is safe to use concurrently.
func (me *MetricsEngine) GetMetricsWithBreachedThresholdsCount() uint32 {
	return atomic.LoadUint32(&me.breachedThresholdsCount)
}
