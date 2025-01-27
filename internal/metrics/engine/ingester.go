package engine

import (
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

const (
	collectRate          = 50 * time.Millisecond
	timeSeriesFirstLimit = 100_000
)

var _ output.Output = &OutputIngester{}

// IngesterDescription is a short description for ingester.
// This variable is used from a function in cmd/ui file for matching this output
// and print a special text.
const IngesterDescription = "Internal Metrics Ingester"

// OutputIngester implements the output.Output interface and can be used to
// "feed" the MetricsEngine data from a `k6 run` test run.
type OutputIngester struct {
	output.SampleBuffer
	logger logrus.FieldLogger

	metricsEngine   *MetricsEngine
	periodicFlusher *output.PeriodicFlusher
	cardinality     *cardinalityControl
}

// Description returns a human-readable description of the output.
func (oi *OutputIngester) Description() string {
	return IngesterDescription
}

// Start the engine by initializing a new output.PeriodicFlusher
func (oi *OutputIngester) Start() error {
	oi.logger.Debug("Starting...")

	pf, err := output.NewPeriodicFlusher(collectRate, oi.flushMetrics)
	if err != nil {
		return err
	}
	oi.logger.Debug("Started!")
	oi.periodicFlusher = pf

	return nil
}

// Stop flushes any remaining metrics and stops the goroutine.
func (oi *OutputIngester) Stop() error {
	oi.logger.Debug("Stopping...")
	defer oi.logger.Debug("Stopped!")
	oi.periodicFlusher.Stop()
	return nil
}

// flushMetrics Writes samples to the MetricsEngine
func (oi *OutputIngester) flushMetrics() {
	sampleContainers := oi.GetBufferedSamples()
	if len(sampleContainers) == 0 {
		return
	}

	oi.metricsEngine.MetricsLock.Lock()
	defer oi.metricsEngine.MetricsLock.Unlock()

	// TODO: split metric samples in buckets with a *metrics.Metric key; this will
	// allow us to have a per-bucket lock, instead of one global one, and it
	// will allow us to split apart the metric Name and Type from its Sink and
	// Observed fields...
	//
	// And, to further optimize things, if every metric (and sub-metric) had a
	// sequential integer ID, we would be able to use a slice for these buckets
	// and eliminate the map loopkups altogether!

	for _, sampleContainer := range sampleContainers {
		samples := sampleContainer.GetSamples()

		if len(samples) == 0 {
			continue
		}

		for _, sample := range samples {
			m := sample.Metric               // this should have come from the Registry, no need to look it up
			oi.metricsEngine.markObserved(m) // mark it as observed so it shows in the end-of-test summary
			m.Sink.Add(sample)               // finally, add its value to its own sink

			// and also to the same for any submetrics that match the metric sample
			for _, sm := range m.Submetrics {
				if !sample.Tags.Contains(sm.Tags) {
					continue
				}
				oi.metricsEngine.markObserved(sm.Metric)
				sm.Metric.Sink.Add(sample)
			}

			oi.cardinality.Add(sample.TimeSeries)
		}
	}

	if oi.cardinality.LimitHit() {
		// TODO: suggest using the Metadata API as an alternative, once it's
		// available (e.g. move high-cardinality tags as Metadata)
		// https://github.com/grafana/k6/issues/2766

		oi.logger.Warnf(
			"The test has generated metrics with %d unique time series, "+
				"which is higher than the suggested limit of %d "+
				"and could cause high memory usage. "+
				"Consider not using high-cardinality values like unique IDs as metric tags "+
				"or, if you need them in the URL, use the name metric tag or URL grouping. "+
				"See https://grafana.com/docs/k6/latest/using-k6/tags-and-groups/ for details.",
			oi.cardinality.Count(),
			timeSeriesFirstLimit,
		)
	}
}

type cardinalityControl struct {
	seen            map[metrics.TimeSeries]struct{}
	timeSeriesLimit int
}

func newCardinalityControl() *cardinalityControl {
	return &cardinalityControl{
		timeSeriesLimit: timeSeriesFirstLimit,
		seen:            make(map[metrics.TimeSeries]struct{}),
	}
}

// Add adds the passed time series to the list of seen items.
func (cc *cardinalityControl) Add(ts metrics.TimeSeries) {
	if _, ok := cc.seen[ts]; ok {
		return
	}
	cc.seen[ts] = struct{}{}
}

// LimitHit checks if the cardinality limit has been hit.
func (cc *cardinalityControl) LimitHit() bool {
	if len(cc.seen) <= cc.timeSeriesLimit {
		return false
	}

	// we don't care about overflow
	// the process should be already OOM
	// if the number of generated time series goes higher than N-hundred-million(s).
	cc.timeSeriesLimit *= 2
	return true
}

// Count returns the number of distinct seen time series.
func (cc *cardinalityControl) Count() int {
	return len(cc.seen)
}
