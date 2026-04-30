// Package expv2 contains a Cloud output using a Protobuf
// binary format for encoding payloads.
package expv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/build"
	"go.k6.io/k6/v2/internal/cloudapi/insights"
	insightsOutput "go.k6.io/k6/v2/internal/output/cloud/insights"
	"go.k6.io/k6/v2/metrics"
	"go.k6.io/k6/v2/output"

	"github.com/sirupsen/logrus"
)

// flusher is an interface for flushing data to the cloud.
type flusher interface {
	flush(ctx context.Context) error
}

// Output sends result data to the k6 Cloud service.
type Output struct {
	output.SampleBuffer

	logger      logrus.FieldLogger
	config      cloudapi.Config
	cloudClient *cloudapi.Client
	testRunID   string

	collector *collector
	flushing  flusher

	insightsClient            insightsOutput.Client
	requestMetadatasCollector insightsOutput.RequestMetadatasCollector
	requestMetadatasFlusher   insightsOutput.RequestMetadatasFlusher

	// wg tracks background goroutines
	wg sync.WaitGroup

	// stop signal to graceful stop
	stop chan struct{}

	// flushCtx scopes the lifetime of in-flight flush HTTP requests.
	// Canceling it during shutdown unblocks pushes whose response
	// is hanging on the network so wg.Wait() can return promptly.
	flushCtx    context.Context
	flushCancel context.CancelFunc

	// abort signal to interrupt immediately all background goroutines
	abort        chan struct{}
	abortOnce    sync.Once
	testStopFunc func(error)
}

// New creates a new cloud output.
func New(logger logrus.FieldLogger, conf cloudapi.Config, _ *cloudapi.Client) (*Output, error) {
	// TODO: move this creation operation to the centralized output. Reducing the probability to
	// break the logic for the config overwriting.
	//
	// It creates a new client because in the case the backend has overwritten
	// the config we need to use the new set.
	o := &Output{
		config: conf,
		logger: logger.WithField("output", "cloudv2"),
		abort:  make(chan struct{}),
		stop:   make(chan struct{}),
		cloudClient: cloudapi.NewClient(
			logger, conf.Token.String, conf.Host.String, build.Version, conf.Timeout.TimeDuration()),
	}
	o.flushCtx, o.flushCancel = context.WithCancel(context.Background())
	return o, nil
}

// ensureFlushCtx initializes flushCtx and flushCancel lazily. It exists
// because some tests construct Output{} directly without going through
// New() and still exercise flushMetrics via the periodic flush path.
func (o *Output) ensureFlushCtx() {
	if o.flushCtx == nil {
		o.flushCtx, o.flushCancel = context.WithCancel(context.Background())
	}
}

// SetTestRunID sets the Cloud's test run id.
func (o *Output) SetTestRunID(id string) {
	o.testRunID = id
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
	defer o.logger.Debug("Started!")

	var err error
	o.collector, err = newCollector(
		o.config.AggregationPeriod.TimeDuration(),
		o.config.AggregationWaitPeriod.TimeDuration())
	if err != nil {
		return fmt.Errorf("failed to initialize the samples collector: %w", err)
	}

	mc, err := newMetricsClient(o.cloudClient, o.testRunID)
	if err != nil {
		return fmt.Errorf("failed to initialize the http metrics flush client: %w", err)
	}
	o.flushing = &metricsFlusher{
		testRunID:                  o.testRunID,
		bq:                         &o.collector.bq,
		client:                     mc,
		logger:                     o.logger,
		discardedLabels:            make(map[string]struct{}),
		aggregationPeriodInSeconds: uint32(o.config.AggregationPeriod.TimeDuration().Seconds()),
		maxSeriesInBatch:           int(o.config.MaxTimeSeriesInBatch.Int64),
		// TODO: when the migration from v1 is over
		// change the default of cloudapi.MetricPushConcurrency to use GOMAXPROCS(0)
		batchPushConcurrency: int(o.config.MetricPushConcurrency.Int64),
	}

	o.runPeriodicFlush()
	o.periodicInvoke(o.config.AggregationPeriod.TimeDuration(), o.collectSamples)

	if insightsOutput.Enabled(o.config) {
		testRunID, err := strconv.ParseInt(o.testRunID, 10, 64)
		if err != nil {
			return err
		}
		o.requestMetadatasCollector = insightsOutput.NewCollector(testRunID)

		insightsClientConfig := insights.NewDefaultClientConfigForTestRun(
			o.config.TracesHost.String,
			o.config.Token.String,
			testRunID,
		)
		insightsClient := insights.NewClient(insightsClientConfig)

		if err := insightsClient.Dial(context.Background()); err != nil {
			return err
		}

		o.insightsClient = insightsClient
		o.requestMetadatasFlusher = insightsOutput.NewFlusher(insightsClient, o.requestMetadatasCollector)
		o.runFlushRequestMetadatas()
	}

	o.logger.WithField("config", printableConfig(o.config)).Debug("Started!")

	return nil
}

// StopWithTestError gracefully stops all metric emission from the output.
func (o *Output) StopWithTestError(_ error) error {
	o.logger.Debug("Stopping...")
	defer o.logger.Debug("Stopped!")

	o.ensureFlushCtx()
	close(o.stop)

	// Wait for background flush loops to exit. If a flush is hung on a
	// network request, give it a short grace period and then cancel
	// flushCtx so the in-flight HTTP push returns and the wait group
	// can complete. Otherwise k6 may hang on shutdown long enough that
	// k6agent forcefully terminates the process with SIGQUIT
	// (k6-cloud-agent issue #341).
	const inflightFlushGrace = 5 * time.Second
	wgDone := make(chan struct{})
	go func() {
		o.wg.Wait()
		close(wgDone)
	}()
	flushCanceledOnTimeout := false
	select {
	case <-wgDone:
	case <-time.After(inflightFlushGrace):
		o.logger.Warn("Cloud output flush did not return in time on shutdown, canceling in-flight requests")
		o.flushCancel()
		flushCanceledOnTimeout = true
		<-wgDone
	}

	select {
	case <-o.abort:
		return nil
	default:
	}

	// If we had to cancel an in-flight push to escape the wait group,
	// the network is unhealthy. Skip the final flush (which would also
	// hang) and let the caller proceed to testFinished.
	if flushCanceledOnTimeout {
		return nil
	}

	// Drain the SampleBuffer and force the aggregation for flushing
	// all the queued samples even if they haven't yet passed the
	// wait period. Use a fresh context so the final flush is not
	// poisoned by a previously canceled flushCtx.
	finalCtx, finalCancel := context.WithTimeout(context.Background(), o.config.Timeout.TimeDuration())
	defer finalCancel()
	o.collector.DropExpiringDelay()
	o.collectSamples()
	o.flushMetrics(finalCtx)

	// Flush all the remaining request metadatas.
	if insightsOutput.Enabled(o.config) {
		o.flushRequestMetadatas()
		if err := o.insightsClient.Close(); err != nil {
			o.logger.WithError(err).Error("Failed to close the insights client")
		}
	}

	return nil
}

func (o *Output) runPeriodicFlush() {
	o.ensureFlushCtx()
	t := time.NewTicker(o.config.MetricPushInterval.TimeDuration())

	o.wg.Add(1)

	go func() {
		defer func() {
			t.Stop()
			o.wg.Done()
		}()

		for {
			select {
			case <-t.C:
				o.flushMetrics(o.flushCtx)
			case <-o.stop:
				return
			case <-o.abort:
				return
			}
		}
	}()
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
	o.wg.Go(func() {
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
	})
}

func (o *Output) collectSamples() {
	samples := o.GetBufferedSamples()
	o.collector.CollectSamples(samples)

	if insightsOutput.Enabled(o.config) {
		o.requestMetadatasCollector.CollectRequestMetadatas(samples)
	}
}

// flushMetrics receives a set of metric samples.
func (o *Output) flushMetrics(ctx context.Context) {
	start := time.Now()

	err := o.flushing.flush(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		o.handleFlushError(err)
		return
	}

	o.logger.WithField("t", time.Since(start)).Trace("Successfully flushed buffered samples to the cloud")
}

func (o *Output) runFlushRequestMetadatas() {
	t := time.NewTicker(o.config.TracesPushInterval.TimeDuration())

	for i := int64(0); i < o.config.TracesPushConcurrency.Int64; i++ {
		o.wg.Go(func() {
			defer t.Stop()

			for {
				select {
				case <-t.C:
					o.flushRequestMetadatas()
				case <-o.stop:
					return
				case <-o.abort:
					return
				}
			}
		})
	}
}

// flushRequestMetadatas periodically flushes traces collected in RequestMetadatasCollector using flusher.
func (o *Output) flushRequestMetadatas() {
	start := time.Now()

	err := o.requestMetadatasFlusher.Flush()
	if err != nil {
		o.logger.WithError(err).WithField("t", time.Since(start)).Error("Failed to push trace samples to the cloud")

		return
	}

	o.logger.WithField("t", time.Since(start)).Trace("Successfully flushed buffered trace samples to the cloud")
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
	// Don't actually handle any errors if we were aborted
	select {
	case <-o.abort:
		return
	default:
	}

	o.logger.WithError(err).Error("Failed to push metrics to the cloud")

	var errResp cloudapi.ResponseError
	if !errors.As(err, &errResp) || errResp.Response == nil {
		return
	}
	// The Cloud service returns the error code 4 when it doesn't accept any more metrics.
	// So, when k6 sees that, the cloud output just stops prematurely.
	if errResp.Response.StatusCode != http.StatusForbidden || errResp.Code != 4 {
		return
	}

	// Do not close multiple times (that would panic) in the case
	// we hit this multiple times and/or concurrently
	o.abortOnce.Do(func() {
		o.logger.WithError(err).Warn("Interrupt sending metrics to cloud due to an error")

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

func printableConfig(c cloudapi.Config) map[string]any {
	m := map[string]any{
		"host":                  c.Host.String,
		"name":                  c.Name.String,
		"timeout":               c.Timeout.String(),
		"webAppURL":             c.WebAppURL.String,
		"projectID":             c.ProjectID.Int64,
		"pushRefID":             c.PushRefID.String,
		"stopOnError":           c.StopOnError.Bool,
		"testRunDetails":        c.TestRunDetails.String,
		"aggregationPeriod":     c.AggregationPeriod.String(),
		"aggregationWaitPeriod": c.AggregationWaitPeriod.String(),
		"maxTimeSeriesInBatch":  c.MaxTimeSeriesInBatch.Int64,
		"metricPushConcurrency": c.MetricPushConcurrency.Int64,
		"metricPushInterval":    c.MetricPushInterval.String(),
		"token":                 "",
	}

	if c.Token.Valid {
		m["token"] = "***"
	}

	return m
}
