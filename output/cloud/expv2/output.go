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

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cloudapi/insights"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"

	"github.com/sirupsen/logrus"
)

// requestMetadatasCollector is an interface for collecting request metadatas
// and retrieving them, so they can be flushed using a flusher.
type requestMetadatasCollector interface {
	CollectRequestMetadatas([]metrics.SampleContainer)
	PopAll() insights.RequestMetadatas
}

// flusher is an interface for flushing data to the cloud.
type flusher interface {
	flush() error
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

	insightsClient            insightsClient
	requestMetadatasCollector requestMetadatasCollector
	requestMetadatasFlusher   flusher

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
func New(logger logrus.FieldLogger, conf cloudapi.Config, _ *cloudapi.Client) (*Output, error) {
	return &Output{
		config: conf,
		logger: logger.WithField("output", "cloudv2"),
		abort:  make(chan struct{}),
		stop:   make(chan struct{}),

		// TODO: move this creation operation to the centralized output. Reducing the probability to
		// break the logic for the config overwriting.
		//
		// It creates a new client because in the case the backend has overwritten
		// the config we need to use the new set.
		cloudClient: cloudapi.NewClient(
			logger, conf.Token.String, conf.Host.String, consts.Version, conf.Timeout.TimeDuration()),
	}, nil
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
	}

	o.runFlushWorkers()
	o.periodicInvoke(o.config.AggregationPeriod.TimeDuration(), o.collectSamples)

	if o.tracingEnabled() {
		testRunID, err := strconv.ParseInt(o.testRunID, 10, 64)
		if err != nil {
			return err
		}
		o.requestMetadatasCollector = newRequestMetadatasCollector(testRunID)

		insightsClientConfig := insights.ClientConfig{
			IngesterHost: o.config.TracesHost.String,
			Timeout:      types.NewNullDuration(90*time.Second, false),
			AuthConfig: insights.ClientAuthConfig{
				Enabled:                  true,
				TestRunID:                testRunID,
				Token:                    o.config.Token.String,
				RequireTransportSecurity: true,
			},
			TLSConfig: insights.ClientTLSConfig{
				Insecure: false,
			},
			RetryConfig: insights.ClientRetryConfig{
				RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`,
				MaxAttempts:          3,
				PerRetryTimeout:      30 * time.Second,
				BackoffConfig: insights.ClientBackoffConfig{
					Enabled:        true,
					JitterFraction: 0.1,
					WaitBetween:    1 * time.Second,
				},
			},
		}
		insightsClient := insights.NewClient(insightsClientConfig)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := insightsClient.Dial(ctx); err != nil {
			return err
		}

		o.insightsClient = insightsClient
		o.requestMetadatasFlusher = newTracesFlusher(insightsClient, o.requestMetadatasCollector)
		o.runFlushRequestMetadatas()
	}

	o.logger.WithField("config", printableConfig(o.config)).Debug("Started!")

	return nil
}

// StopWithTestError gracefully stops all metric emission from the output.
func (o *Output) StopWithTestError(_ error) error {
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

	// Flush all the remaining request metadatas.
	if o.tracingEnabled() {
		o.flushRequestMetadatas()
		if err := o.insightsClient.Close(); err != nil {
			o.logger.WithError(err).Error("Failed to close the insights client")
		}
	}

	return nil
}

func (o *Output) runFlushWorkers() {
	t := time.NewTicker(o.config.MetricPushInterval.TimeDuration())

	for i := int64(0); i < o.config.MetricPushConcurrency.Int64; i++ {
		o.wg.Add(1)
		go func() {
			defer func() {
				t.Stop()
				o.wg.Done()
			}()

			for {
				select {
				case <-t.C:
					o.flushMetrics()
				case <-o.stop:
					return
				case <-o.abort:
					return
				}
			}
		}()
	}
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

	if o.tracingEnabled() {
		o.requestMetadatasCollector.CollectRequestMetadatas(samples)
	}
}

// flushMetrics receives a set of metric samples.
func (o *Output) flushMetrics() {
	start := time.Now()

	err := o.flushing.flush()
	if err != nil {
		o.handleFlushError(err)
		return
	}

	o.logger.WithField("t", time.Since(start)).Debug("Successfully flushed buffered samples to the cloud")
}

func (o *Output) runFlushRequestMetadatas() {
	t := time.NewTicker(o.config.TracesPushInterval.TimeDuration())

	for i := int64(0); i < o.config.TracesPushConcurrency.Int64; i++ {
		o.wg.Add(1)
		go func() {
			defer o.wg.Done()
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
		}()
	}
}

// flushRequestMetadatas periodically flushes traces collected in RequestMetadatasCollector using flusher.
func (o *Output) flushRequestMetadatas() {
	start := time.Now()

	err := o.requestMetadatasFlusher.flush()
	if err != nil {
		o.logger.WithError(err).WithField("t", time.Since(start)).Error("Failed to push trace samples to the cloud")
	}

	o.logger.WithField("t", time.Since(start)).Debug("Successfully flushed buffered trace samples to the cloud")
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

	var errResp cloudapi.ErrorResponse
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

func (o *Output) tracingEnabled() bool {
	// TODO(lukasz): Check if k6 x Tempo is enabled
	//
	// We want to check whether a given organization is
	// eligible for k6 x Tempo feature. If it isn't, we may
	// consider to skip the traces output.
	//
	// We currently don't have a backend API to check this
	// information.
	return o.config.TracesEnabled.ValueOrZero()
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
