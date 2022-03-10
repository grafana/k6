// Package engine contains the internal metrics engine responsible for
// aggregating metrics during the test and evaluating thresholds against them.
package engine

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
	"gopkg.in/guregu/null.v3"
)

const thresholdsRate = 2 * time.Second

// TODO: move to the main metrics package

// MetricsEngine is the internal metrics engine that k6 uses to keep track of
// aggregated metric sample values. They are used to generate the end-of-test
// summary and to evaluate the test thresholds.
type MetricsEngine struct {
	registry   *metrics.Registry
	thresholds map[string]stats.Thresholds
	logger     logrus.FieldLogger

	outputIngester *outputIngester

	// These can be both top-level metrics or sub-metrics
	metricsWithThresholds []*stats.Metric

	breachedThresholdsCount uint32

	// TODO: completely refactor:
	//   - make these private, add a method to export the raw data
	//   - do not use an unnecessary map for the observed metrics
	//   - have one lock per metric instead of a a global one, when
	//     the metrics are decoupled from their types
	MetricsLock     sync.Mutex
	ObservedMetrics map[string]*stats.Metric
}

// NewMetricsEngine creates a new metrics Engine with the given parameters.
func NewMetricsEngine(
	registry *metrics.Registry, thresholds map[string]stats.Thresholds,
	shouldProcessMetrics, noThresholds bool, systemTags *stats.SystemTagSet, logger logrus.FieldLogger,
) (*MetricsEngine, error) {
	me := &MetricsEngine{
		registry:   registry,
		thresholds: thresholds,
		logger:     logger.WithField("component", "metrics-engine"),

		ObservedMetrics: make(map[string]*stats.Metric),
	}

	if shouldProcessMetrics {
		err := me.initSubMetricsAndThresholds(noThresholds, systemTags)
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

// TODO: something better
func (me *MetricsEngine) ImportMetric(name string, data []byte) error {
	me.MetricsLock.Lock()
	defer me.MetricsLock.Unlock()

	// TODO: replace with strings.Cut after Go 1.18
	nameParts := strings.SplitN(name, "{", 2)

	metric := me.registry.Get(nameParts[0])
	if metric == nil {
		return fmt.Errorf("metric '%s' does not exist in the script", nameParts[0])
	}
	if len(nameParts) == 1 { // no sub-metric
		me.markObserved(metric)
		return metric.Sink.Merge(data)
	}

	if nameParts[1][len(nameParts[1])-1] != '}' {
		return fmt.Errorf("missing ending bracket, sub-metric format needs to be 'metric{key:value}'")
	}

	sm, err := metric.GetSubmetric(nameParts[1][:len(nameParts[1])-1])
	if err != nil {
		return err
	}

	me.markObserved(sm.Metric)
	return sm.Metric.Sink.Merge(data)
}

func (me *MetricsEngine) getOrInitPotentialSubmetric(name string) (*stats.Metric, error) {
	// TODO: replace with strings.Cut after Go 1.18
	nameParts := strings.SplitN(name, "{", 2)

	metric := me.registry.Get(nameParts[0])
	if metric == nil {
		return nil, fmt.Errorf("metric '%s' does not exist in the script", nameParts[0])
	}
	if len(nameParts) == 1 { // no sub-metric
		return metric, nil
	}

	if nameParts[1][len(nameParts[1])-1] != '}' {
		return nil, fmt.Errorf("missing ending bracket, sub-metric format needs to be 'metric{key:value}'")
	}
	sm, err := metric.AddSubmetric(nameParts[1][:len(nameParts[1])-1])
	if err != nil {
		return nil, err
	}
	return sm.Metric, nil
}

func (me *MetricsEngine) markObserved(metric *stats.Metric) {
	if !metric.Observed {
		metric.Observed = true
		me.ObservedMetrics[metric.Name] = metric
	}
}

func (me *MetricsEngine) initSubMetricsAndThresholds(noThresholds bool, systemTags *stats.SystemTagSet) error {
	for metricName, thresholds := range me.thresholds {
		metric, err := me.getOrInitPotentialSubmetric(metricName)

		if noThresholds {
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
			me.markObserved(metric.Sub.Metric)
		}
	}

	// TODO: refactor out of here when https://github.com/grafana/k6/issues/1321
	// lands and there is a better way to enable a metric with tag
	if systemTags.Has(stats.TagExpectedResponse) {
		_, err := me.getOrInitPotentialSubmetric("http_req_duration{expected_response:true}")
		if err != nil {
			return err // shouldn't happen, but ¯\_(ツ)_/¯
		}
	}

	return nil
}

func (me *MetricsEngine) StartThresholdCalculations(
	abortRun execution.TestAbortFunc, getCurrentTestRunDuration func() time.Duration,
) (finalize func() (breached []string)) {
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(thresholdsRate)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				breached, shouldAbort := me.processThresholds(getCurrentTestRunDuration)
				if shouldAbort {
					err := fmt.Errorf(
						"thresholds on metrics %s were breached; at least one has abortOnFail enabled, stopping test prematurely...",
						strings.Join(breached, ", "),
					)
					me.logger.Debug(err.Error())
					err = errext.WithExitCodeIfNone(err, exitcodes.ThresholdsHaveFailed)
					err = lib.WithRunStatusIfNone(err, lib.RunStatusAbortedThreshold)
					abortRun(err)
				}
			case <-stop:
				// TODO: do the final metrics processing here instead of cmd/run.go?
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

		breached, _ := me.processThresholds(getCurrentTestRunDuration)
		return breached
	}
}

// ProcessThresholds processes all of the thresholds.
//
// TODO: refactor, optimize
func (me *MetricsEngine) processThresholds(
	getCurrentTestRunDuration func() time.Duration,
) (breachedThersholds []string, shouldAbort bool) {
	me.MetricsLock.Lock()
	defer me.MetricsLock.Unlock()

	t := getCurrentTestRunDuration()

	me.logger.Debugf("Running thresholds on %d metrics...", len(me.metricsWithThresholds))
	for _, m := range me.metricsWithThresholds {
		if len(m.Thresholds.Thresholds) == 0 {
			// Should not happen, but just in case...
			me.logger.Warnf("Metric %s unexpectedly has no thersholds defined", m.Name)
			continue
		}
		m.Tainted = null.BoolFrom(false)

		succ, err := m.Thresholds.Run(m.Sink, t)
		if err != nil {
			me.logger.WithField("metric", m.Name).WithError(err).Error("Threshold error")
			continue
		}
		if !succ {
			breachedThersholds = append(breachedThersholds, m.Name)
			m.Tainted = null.BoolFrom(true)
			if m.Thresholds.Abort {
				shouldAbort = true
			}
		}
	}
	me.logger.Debugf("Thresholds on %d metrics breached: %v", len(breachedThersholds), breachedThersholds)
	atomic.StoreUint32(&me.breachedThresholdsCount, uint32(len(breachedThersholds)))
	return breachedThersholds, shouldAbort
}

// GetMetricsWithBreachedThresholdsCount returns the number of metrics for which
// the thresholds were breached (failed) during the last processing phase. This
// API is safe to use concurrently.
func (me *MetricsEngine) GetMetricsWithBreachedThresholdsCount() uint32 {
	return atomic.LoadUint32(&me.breachedThresholdsCount)
}
