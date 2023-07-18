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
	"gopkg.in/guregu/null.v3"
)

const thresholdsRate = 2 * time.Second

// MetricsEngine is the internal metrics engine that k6 uses to keep track of
// aggregated metric sample values. They are used to generate the end-of-test
// summary and to evaluate the test thresholds.
type MetricsEngine struct {
	registry *metrics.Registry
	logger   logrus.FieldLogger

	// These can be both top-level metrics or sub-metrics
	metricsWithThresholds   []*metrics.Metric
	breachedThresholdsCount uint32

	// TODO: completely refactor:
	//   - make these private, add a method to export the raw data
	//   - do not use an unnecessary map for the observed metrics
	//   - have one lock per metric instead of a a global one, when
	//     the metrics are decoupled from their types
	MetricsLock     sync.Mutex
	ObservedMetrics map[string]*metrics.Metric
}

// NewMetricsEngine creates a new metrics Engine with the given parameters.
func NewMetricsEngine(registry *metrics.Registry, logger logrus.FieldLogger) (*MetricsEngine, error) {
	me := &MetricsEngine{
		registry:        registry,
		logger:          logger.WithField("component", "metrics-engine"),
		ObservedMetrics: make(map[string]*metrics.Metric),
	}

	return me, nil
}

// CreateIngester returns a pseudo-Output that uses the given metric samples to
// update the engine's inner state.
func (me *MetricsEngine) CreateIngester() *OutputIngester {
	return &OutputIngester{
		logger:        me.logger.WithField("component", "metrics-engine-ingester"),
		metricsEngine: me,
		cardinality:   newCardinalityControl(),
	}
}

func (me *MetricsEngine) getThresholdMetricOrSubmetric(name string) (*metrics.Metric, error) {
	// TODO: replace with strings.Cut after Go 1.18
	nameParts := strings.SplitN(name, "{", 2)

	metric := me.registry.Get(nameParts[0])
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

	if sm.Metric.Observed {
		// Do not repeat warnings for the same sub-metrics
		return sm.Metric, nil
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

func (me *MetricsEngine) markObserved(metric *metrics.Metric) {
	if !metric.Observed {
		metric.Observed = true
		me.ObservedMetrics[metric.Name] = metric
	}
}

// InitSubMetricsAndThresholds parses the thresholds from the test Options and
// initializes both the thresholds themselves, as well as any submetrics that
// were referenced in them.
func (me *MetricsEngine) InitSubMetricsAndThresholds(options lib.Options, onlyLogErrors bool) error {
	for metricName, thresholds := range options.Thresholds {
		metric, err := me.getThresholdMetricOrSubmetric(metricName)

		if onlyLogErrors {
			if err != nil {
				me.logger.WithError(err).Warnf("Invalid metric '%s' in threshold definitions", metricName)
			}
			continue
		}

		if err != nil {
			return fmt.Errorf("invalid metric '%s' in threshold definitions: %w", metricName, err)
		}

		metric.Thresholds = thresholds
		me.metricsWithThresholds = append(me.metricsWithThresholds, metric)

		// Mark the metric (and the parent metric, if we're dealing with a
		// submetric) as observed, so they are shown in the end-of-test summary,
		// even if they don't have any metric samples during the test run
		me.markObserved(metric)
		if metric.Sub != nil {
			me.markObserved(metric.Sub.Parent)
		}
	}

	// TODO: refactor out of here when https://github.com/grafana/k6/issues/1321
	// lands and there is a better way to enable a metric with tag
	if options.SystemTags.Has(metrics.TagExpectedResponse) {
		_, err := me.getThresholdMetricOrSubmetric("http_req_duration{expected_response:true}")
		if err != nil {
			return err // shouldn't happen, but ¯\_(ツ)_/¯
		}
	}

	return nil
}

// StartThresholdCalculations spins up a new goroutine to crunch thresholds and
// returns a callback that will stop the goroutine and finalizes calculations.
func (me *MetricsEngine) StartThresholdCalculations(
	ingester *OutputIngester,
	abortRun func(error),
	getCurrentTestRunDuration func() time.Duration,
) (finalize func() (breached []string)) {
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
						"thresholds on metrics '%s' were crossed; at least one has abortOnFail enabled, stopping test prematurely",
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
		if ingester != nil {
			// Stop the ingester so we don't get any more metrics
			err := ingester.Stop()
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
	me.MetricsLock.Lock()
	defer me.MetricsLock.Unlock()

	t := getCurrentTestRunDuration()

	me.logger.Debugf("Running thresholds on %d metrics...", len(me.metricsWithThresholds))
	for _, m := range me.metricsWithThresholds {
		// If either the metric has no thresholds defined, or its sinks
		// are empty, let's ignore its thresholds execution at this point.
		if len(m.Thresholds.Thresholds) == 0 || (ignoreEmptySinks && m.Sink.IsEmpty()) {
			continue
		}
		m.Tainted = null.BoolFrom(false)

		succ, err := m.Thresholds.Run(m.Sink, t)
		if err != nil {
			me.logger.WithField("metric_name", m.Name).WithError(err).Error("Threshold error")
			continue
		}
		if succ {
			continue // threshold passed
		}
		breachedThresholds = append(breachedThresholds, m.Name)
		m.Tainted = null.BoolFrom(true)
		if m.Thresholds.Abort {
			shouldAbort = true
		}
	}
	if len(breachedThresholds) > 0 {
		sort.Strings(breachedThresholds)
		me.logger.Debugf("Thresholds on %d metrics crossed: %v", len(breachedThresholds), breachedThresholds)
	}
	atomic.StoreUint32(&me.breachedThresholdsCount, uint32(len(breachedThresholds)))
	return breachedThresholds, shouldAbort
}

// GetMetricsWithBreachedThresholdsCount returns the number of metrics for which
// the thresholds were breached (failed) during the last processing phase. This
// API is safe to use concurrently.
func (me *MetricsEngine) GetMetricsWithBreachedThresholdsCount() uint32 {
	return atomic.LoadUint32(&me.breachedThresholdsCount)
}
