// Package engine contains the internal metrics engine responsible for
// aggregating metrics during the test and evaluating thresholds against them.
package engine

import (
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
	"gopkg.in/guregu/null.v3"
)

// MetricsEngine is the internal metrics engine that k6 uses to keep track of
// aggregated metric sample values. They are used to generate the end-of-test
// summary and to evaluate the test thresholds.
type MetricsEngine struct {
	registry       *metrics.Registry
	executionState *lib.ExecutionState
	options        lib.Options
	runtimeOptions lib.RuntimeOptions
	logger         logrus.FieldLogger

	// These can be both top-level metrics or sub-metrics
	metricsWithThresholds []*stats.Metric

	// TODO: completely refactor:
	//   - make these private,
	//   - do not use an unnecessary map for the observed metrics
	//   - have one lock per metric instead of a a global one, when
	//     the metrics are decoupled from their types
	MetricsLock     sync.Mutex
	ObservedMetrics map[string]*stats.Metric
}

// NewMetricsEngine creates a new metrics Engine with the given parameters.
func NewMetricsEngine(
	registry *metrics.Registry, executionState *lib.ExecutionState,
	opts lib.Options, rtOpts lib.RuntimeOptions, logger logrus.FieldLogger,
) (*MetricsEngine, error) {
	me := &MetricsEngine{
		registry:       registry,
		executionState: executionState,
		options:        opts,
		runtimeOptions: rtOpts,
		logger:         logger.WithField("component", "metrics-engine"),

		ObservedMetrics: make(map[string]*stats.Metric),
	}

	if !(me.runtimeOptions.NoSummary.Bool && me.runtimeOptions.NoThresholds.Bool) {
		err := me.initSubMetricsAndThresholds()
		if err != nil {
			return nil, err
		}
	}

	return me, nil
}

// GetIngester returns a pseudo-Output that uses the given metric samples to
// update the engine's inner state.
func (me *MetricsEngine) GetIngester() output.Output {
	return &outputIngester{
		logger:        me.logger.WithField("component", "metrics-engine-ingester"),
		metricsEngine: me,
	}
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

func (me *MetricsEngine) initSubMetricsAndThresholds() error {
	for metricName, thresholds := range me.options.Thresholds {
		metric, err := me.getOrInitPotentialSubmetric(metricName)

		if me.runtimeOptions.NoThresholds.Bool {
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
	if me.options.SystemTags.Has(stats.TagExpectedResponse) {
		_, err := me.getOrInitPotentialSubmetric("http_req_duration{expected_response:true}")
		if err != nil {
			return err // shouldn't happen, but ¯\_(ツ)_/¯
		}
	}

	return nil
}

// ProcessThresholds processes all of the thresholds.
//
// TODO: refactor, make private, optimize
func (me *MetricsEngine) ProcessThresholds() (thresholdsTainted, shouldAbort bool) {
	me.MetricsLock.Lock()
	defer me.MetricsLock.Unlock()

	t := me.executionState.GetCurrentTestRunDuration()

	for _, m := range me.metricsWithThresholds {
		if len(m.Thresholds.Thresholds) == 0 {
			continue
		}
		m.Tainted = null.BoolFrom(false)

		me.logger.WithField("m", m.Name).Debug("running thresholds")
		succ, err := m.Thresholds.Run(m.Sink, t)
		if err != nil {
			me.logger.WithField("m", m.Name).WithError(err).Error("Threshold error")
			continue
		}
		if !succ {
			me.logger.WithField("m", m.Name).Debug("Thresholds failed")
			m.Tainted = null.BoolFrom(true)
			thresholdsTainted = true
			if m.Thresholds.Abort {
				shouldAbort = true
			}
		}
	}

	return thresholdsTainted, shouldAbort
}
