// Package expv2 contains a Cloud output using a Protobuf
// binary format for encoding payloads.
package expv2

import (
	"context"
	"net/http"
	"time"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"

	"github.com/sirupsen/logrus"
)

// Output sends result data to the k6 Cloud service.
type Output struct {
	output.SampleBuffer

	logger       logrus.FieldLogger
	config       cloudapi.Config
	referenceID  string
	testStopFunc func(error)

	// TODO: replace with the real impl
	metricsFlusher  noopFlusher
	periodicFlusher *output.PeriodicFlusher

	collector             *collector
	periodicCollector     *output.PeriodicFlusher
	stopMetricsCollection chan struct{}
}

// New creates a new cloud output.
func New(logger logrus.FieldLogger, conf cloudapi.Config) (*Output, error) {
	return &Output{
		config:                conf,
		logger:                logger.WithFields(logrus.Fields{"output": "cloudv2"}),
		stopMetricsCollection: make(chan struct{}),
	}, nil
}

// SetReferenceID sets the Cloud's test run ID.
func (o *Output) SetReferenceID(refID string) {
	o.referenceID = refID
}

// SetTestRunStopCallback receives the function that
// that stops the engine when it is called.
// It should be called on critical errors.
func (o *Output) SetTestRunStopCallback(stopFunc func(error)) {
	o.testStopFunc = stopFunc
}

// Start starts the goroutine that would listen
// for metric samples and send them to the cloud.
func (o *Output) Start() error {
	o.logger.Debug("Starting...")

	var err error
	o.collector, err = newCollector(
		o.config.AggregationPeriod.TimeDuration(),
		o.config.AggregationWaitPeriod.TimeDuration())
	if err != nil {
		return err
	}
	o.metricsFlusher = noopFlusher{
		referenceID: o.referenceID,
		bq:          &o.collector.bq,
	}

	pf, err := output.NewPeriodicFlusher(
		o.config.MetricPushInterval.TimeDuration(), o.flushMetrics)
	if err != nil {
		return err
	}
	o.periodicFlusher = pf

	pfc, err := output.NewPeriodicFlusher(
		o.config.AggregationPeriod.TimeDuration(), o.collectSamples)
	if err != nil {
		return err
	}
	o.periodicCollector = pfc

	o.logger.Debug("Started!")
	return nil
}

// StopWithTestError gracefully stops all metric emission from the output.
func (o *Output) StopWithTestError(testErr error) error {
	o.logger.Debug("Stopping...")
	close(o.stopMetricsCollection)

	// Drain the SampleBuffer and force the aggregation for flushing
	// all the queued samples even if they haven't yet passed the
	// wait period.
	o.periodicCollector.Stop()
	o.collector.DropExpiringDelay()
	o.collector.CollectSamples(nil)
	o.periodicFlusher.Stop()

	o.logger.Debug("Stopped!")
	return nil
}

// AddMetricSamples receives the samples streaming.
func (o *Output) AddMetricSamples(s []metrics.SampleContainer) {
	// TODO: this and the next operation are two locking operations,
	// evaluate to do something smarter, maybe having a lock-free
	// queue.
	select {
	case <-o.stopMetricsCollection:
		return
	default:
	}

	// TODO: when we will have a very good optimized
	// bucketing process we may evaluate to drop this
	// buffer.
	//
	// If the bucketing process is efficient, the single
	// operation could be a bit longer than just enqueuing
	// but it could be fast enough to justify to to direct
	// run it and save some memory across the e2e operation.
	//
	// It requires very specific benchmark.
	o.SampleBuffer.AddMetricSamples(s)
}

func (o *Output) collectSamples() {
	samples := o.GetBufferedSamples()
	if len(samples) < 1 {
		return
	}
	o.collector.CollectSamples(samples)

	// TODO: other operations with the samples containers
	// e.g. flushing the Metadata as tracing samples
}

// flushMetrics receives a set of metric samples.
func (o *Output) flushMetrics() {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), o.config.MetricPushInterval.TimeDuration())
	defer cancel()

	err := o.metricsFlusher.Flush(ctx)
	if err != nil {
		o.logger.WithError(err).Error("Failed to push metrics to the cloud")

		if o.shouldStopSendingMetrics(err) {
			o.logger.WithError(err).Warn("Interrupt sending metrics to cloud due to an error")
			serr := errext.WithAbortReasonIfNone(
				errext.WithExitCodeIfNone(err, exitcodes.ExternalAbort),
				errext.AbortedByOutput,
			)
			if o.config.StopOnError.Bool {
				o.testStopFunc(serr)
			}
			close(o.stopMetricsCollection)
		}
		return
	}

	o.logger.WithField("t", time.Since(start)).Debug("Successfully flushed buffered samples to the cloud")
}

// shouldStopSendingMetrics returns true if the output should interrupt the metric flush.
//
// note: The actual test execution should continues,
// since for local k6 run tests the end-of-test summary (or any other outputs) will still work,
// but the cloud output doesn't send any more metrics.
// Instead, if cloudapi.Config.StopOnError is enabled
// the cloud output should stop the whole test run too.
// This logic should be handled by the caller.
func (o *Output) shouldStopSendingMetrics(err error) bool {
	if err == nil {
		return false
	}
	if errResp, ok := err.(cloudapi.ErrorResponse); ok && errResp.Response != nil { //nolint:errorlint
		// The Cloud service returns the error code 4 when it doesn't accept any more metrics.
		// So, when k6 sees that, the cloud output just stops prematurely.
		return errResp.Response.StatusCode == http.StatusForbidden && errResp.Code == 4
	}

	return false
}
