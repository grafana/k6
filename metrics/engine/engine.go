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
	"gopkg.in/guregu/null.v3"
)

// MetricsEngine is the internal metrics engine that k6 uses to keep track of
// aggregated metric sample values. They are used to generate the end-of-test
// summary and to evaluate the test thresholds.
type MetricsEngine struct {
	es     *lib.ExecutionState
	logger logrus.FieldLogger

	// These can be both top-level metrics or sub-metrics
	metricsWithThresholds []*metrics.Metric

	// TODO: completely refactor:
	//   - make these private,
	//   - do not use an unnecessary map for the observed metrics
	//   - have one lock per metric instead of a a global one, when
	//     the metrics are decoupled from their types
	MetricsLock     sync.Mutex
	ObservedMetrics map[string]*metrics.Metric
}

// NewMetricsEngine creates a new metrics Engine with the given parameters.
func NewMetricsEngine(es *lib.ExecutionState) (*MetricsEngine, error) {
	me := &MetricsEngine{
		es:     es,
		logger: es.Test.Logger.WithField("component", "metrics-engine"),

		ObservedMetrics: make(map[string]*metrics.Metric),
	}

	if !(me.es.Test.RuntimeOptions.NoSummary.Bool && me.es.Test.RuntimeOptions.NoThresholds.Bool) {
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

func (me *MetricsEngine) getThresholdMetricOrSubmetric(name string) (*metrics.Metric, error) {
	// TODO: replace with strings.Cut after Go 1.18
	nameParts := strings.SplitN(name, "{", 2)

	metric := me.es.Test.Registry.Get(nameParts[0])
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

	// TODO: reword these from "will be deprecated" to "were deprecated" and
	// maybe make them errors, not warnings, when we introduce un-indexable tags

	if _, ok := sm.Tags.Get("url"); ok {
		me.logger.Warnf("Thresholds like '%s', based on the high-cardinality 'url' metric tag, "+
			"are deprecated and will not be supported in future k6 releases. "+
			"To prevent breaking changes and reduce bugs, use the 'name' metric tag instead, see"+
			"URL grouping (https://k6.io/docs/using-k6/http-requests/#url-grouping) for more information.", name,
		)
	}

	if _, ok := sm.Tags.Get("error"); ok {
		me.logger.Warnf("Thresholds like '%s', based on the high-cardinality 'error' metric tag, "+
			"are deprecated and will not be supported in future k6 releases. "+
			"To prevent breaking changes and reduce bugs, use the 'error_code' metric tag instead", name,
		)
	}
	if _, ok := sm.Tags.Get("vu"); ok {
		me.logger.Warnf("Thresholds like '%s', based on the high-cardinality 'vu' metric tag, "+
			"are deprecated and will not be supported in future k6 releases.", name,
		)
	}

	if _, ok := sm.Tags.Get("iter"); ok {
		me.logger.Warnf("Thresholds like '%s', based on the high-cardinality 'iter' metric tag, "+
			"are deprecated and will not be supported in future k6 releases.", name,
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

func (me *MetricsEngine) initSubMetricsAndThresholds() error {
	for metricName, thresholds := range me.es.Test.Options.Thresholds {
		metric, err := me.getThresholdMetricOrSubmetric(metricName)

		if me.es.Test.RuntimeOptions.NoThresholds.Bool {
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
	if me.es.Test.Options.SystemTags.Has(metrics.TagExpectedResponse) {
		_, err := me.getThresholdMetricOrSubmetric("http_req_duration{expected_response:true}")
		if err != nil {
			return err // shouldn't happen, but ¯\_(ツ)_/¯
		}
	}

	return nil
}

// EvaluateThresholds processes all of the thresholds.
//
// TODO: refactor, make private, optimize
func (me *MetricsEngine) EvaluateThresholds(ignoreEmptySinks bool) (thresholdsTainted, shouldAbort bool) {
	me.MetricsLock.Lock()
	defer me.MetricsLock.Unlock()

	t := me.es.GetCurrentTestRunDuration()

	for _, m := range me.metricsWithThresholds {
		// If either the metric has no thresholds defined, or its sinks
		// are empty, let's ignore its thresholds execution at this point.
		if len(m.Thresholds.Thresholds) == 0 || (ignoreEmptySinks && m.Sink.IsEmpty()) {
			continue
		}
		m.Tainted = null.BoolFrom(false)

		me.logger.WithField("metric_name", m.Name).Debug("running thresholds")
		succ, err := m.Thresholds.Run(m.Sink, t)
		if err != nil {
			me.logger.WithField("metric_name", m.Name).WithError(err).Error("Threshold error")
			continue
		}
		if succ {
			continue // threshold passed
		}
		me.logger.WithField("metric_name", m.Name).Debug("Thresholds failed")
		m.Tainted = null.BoolFrom(true)
		thresholdsTainted = true
		if m.Thresholds.Abort {
			shouldAbort = true
		}
	}

	return thresholdsTainted, shouldAbort
}
