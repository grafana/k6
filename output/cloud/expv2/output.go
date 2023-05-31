// Package expv2 contains a Cloud output using a Protobuf
// binary format for encoding payloads.
package expv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
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

	logger      logrus.FieldLogger
	config      cloudapi.Config
	referenceID string

	collector *collector
	flushing  *metricsFlusher

	// wg tracks background goroutines
	wg sync.WaitGroup

	// stop signal to graceful stop
	stop chan struct{}

	// abort signal to interrupt immediately all background goroutines
	abort        chan struct{}
	abortOnce    sync.Once
	testStopFunc func(error)
}

// New creates a new cloud output.
func New(logger logrus.FieldLogger, conf cloudapi.Config) (*Output, error) {
	return &Output{
		config: conf,
		logger: logger.WithFields(logrus.Fields{"output": "cloudv2"}),
		abort:  make(chan struct{}),
		stop:   make(chan struct{}),
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
		return fmt.Errorf("failed to initialize the samples collector: %w", err)
	}

	mc, err := newMetricsClient(o.logger, o.config.Host.String, o.config.Token.String)
	if err != nil {
		return fmt.Errorf("failed to initialize the http metrics flush client: %w", err)
	}
	o.flushing = &metricsFlusher{
		referenceID:                o.referenceID,
		bq:                         &o.collector.bq,
		client:                     mc,
		aggregationPeriodInSeconds: uint32(o.config.AggregationPeriod.TimeDuration().Seconds()),
	}

	o.periodicInvoke(o.config.MetricPushInterval.TimeDuration(), o.flushMetrics)
	o.periodicInvoke(o.config.AggregationPeriod.TimeDuration(), o.collectSamples)

	o.logger.Debug("Started!")
	return nil
}

// StopWithTestError gracefully stops all metric emission from the output.
func (o *Output) StopWithTestError(testErr error) error {
	o.logger.Debug("Stopping...")
	defer o.logger.Debug("Stopped!")

	close(o.stop)
	o.wg.Wait()

	select {
	case <-o.abort:
		return nil
	default:
	}

	// Drain the SampleBuffer and force the aggregation for flushing
	// all the queued samples even if they haven't yet passed the
	// wait period.
	o.collector.DropExpiringDelay()
	o.collectSamples()
	o.flushMetrics()

	return nil
}

// AddMetricSamples receives the samples streaming.
func (o *Output) AddMetricSamples(s []metrics.SampleContainer) {
	// TODO: this and the next operation are two locking operations,
	// evaluate to do something smarter, maybe having a lock-free
	// queue.
	select {
	case <-o.abort:
		return
	default:
	}

	// TODO: when we will have a very good optimized
	// bucketing process we may evaluate to drop this
	// buffer.
	//
	// If the bucketing process is efficient, the single
	// operation could be a bit longer than just enqueuing
	// but it could be fast enough to justify to direct
	// run it and save some memory across the e2e operation.
	//
	// It requires very specific benchmark.
	o.SampleBuffer.AddMetricSamples(s)
}

func (o *Output) periodicInvoke(d time.Duration, callback func()) {
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()

		t := time.NewTicker(d)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				callback()
			case <-o.stop:
				return
			case <-o.abort:
				return
			}
		}
	}()
}

func (o *Output) collectSamples() {
	samples := o.GetBufferedSamples()
	o.collector.CollectSamples(samples)

	// TODO: other operations with the samples containers
	// e.g. flushing the Metadata as tracing samples
}

// flushMetrics receives a set of metric samples.
func (o *Output) flushMetrics() {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), o.config.MetricPushInterval.TimeDuration())
	defer cancel()

	err := o.flushing.Flush(ctx)
	if err != nil {
		o.handleFlushError(err)
		return
	}

	o.logger.WithField("t", time.Since(start)).Debug("Successfully flushed buffered samples to the cloud")
}

// handleFlushError handles errors generated from the flushing operation.
// It may interrupt the metric collection or invoke aborting of the test.
//
// note: The actual test execution should continue, since for local k6 run tests
// the end-of-test summary (or any other outputs) will still work,
// but the cloud output doesn't send any more metrics.
// Instead, if cloudapi.Config.StopOnError is enabled the cloud output should
// stop the whole test run too. This logic should be handled by the caller.
func (o *Output) handleFlushError(err error) {
	o.logger.WithError(err).Error("Failed to push metrics to the cloud")

	var errResp cloudapi.ErrorResponse
	if !errors.As(err, &errResp) || errResp.Response == nil {
		return
	}

	// The Cloud service returns the error code 4 when it doesn't accept any more metrics.
	// So, when k6 sees that, the cloud output just stops prematurely.
	if errResp.Response.StatusCode != http.StatusForbidden || errResp.Code != 4 {
		return
	}

	o.logger.WithError(err).Warn("Interrupt sending metrics to cloud due to an error")

	// Do not close multiple times (that would panic) in the case
	// we hit this multiple times and/or concurrently
	o.abortOnce.Do(func() {
		close(o.abort)

		if o.config.StopOnError.Bool {
			serr := errext.WithAbortReasonIfNone(
				errext.WithExitCodeIfNone(err, exitcodes.ExternalAbort),
				errext.AbortedByOutput,
			)

			if o.testStopFunc != nil {
				o.testStopFunc(serr)
			}
		}
	})
}
