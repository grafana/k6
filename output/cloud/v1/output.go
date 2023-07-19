// Package cloud implements an Output that flushes to the k6 Cloud platform
// using the version1 of the protocol flushing a json-based payload.
package cloud

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/mailru/easyjson"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cloudapi/insights"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	insightsOutput "go.k6.io/k6/output/cloud/insights"

	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/metrics"
)

// Output sends result data to the k6 Cloud service.
type Output struct {
	logger logrus.FieldLogger
	config cloudapi.Config

	// referenceID is the legacy name used by the Backend for the test run id.
	// Note: This output's version is almost deprecated so we don't apply
	// the renaming to its internals.
	referenceID string
	client      *MetricsClient

	bufferMutex      sync.Mutex
	bufferHTTPTrails []*httpext.Trail
	bufferSamples    []*Sample

	insightsClient            insightsOutput.Client
	requestMetadatasCollector insightsOutput.RequestMetadatasCollector
	requestMetadatasFlusher   insightsOutput.RequestMetadatasFlusher

	// TODO: optimize this
	//
	// Since the real-time metrics refactoring (https://github.com/k6io/k6/pull/678),
	// we should no longer have to handle metrics that have times long in the past. So instead of a
	// map, we can probably use a simple slice (or even an array!) as a ring buffer to store the
	// aggregation buckets. This should save us a some time, since it would make the lookups and WaitPeriod
	// checks basically O(1). And even if for some reason there are occasional metrics with past times that
	// don't fit in the chosen ring buffer size, we could just send them along to the buffer unaggregated
	aggrBuckets map[int64]aggregationBucket

	stopSendingMetrics chan struct{}
	stopAggregation    chan struct{}
	aggregationDone    *sync.WaitGroup
	stopOutput         chan struct{}
	outputDone         *sync.WaitGroup
	testStopFunc       func(error)
}

// New creates a new Cloud output version 1.
func New(logger logrus.FieldLogger, conf cloudapi.Config, testAPIClient *cloudapi.Client) (*Output, error) {
	return &Output{
		config:      conf,
		client:      NewMetricsClient(testAPIClient, logger, conf.Host.String, conf.NoCompress.Bool),
		aggrBuckets: map[int64]aggregationBucket{},
		logger:      logger,

		stopSendingMetrics: make(chan struct{}),
		stopAggregation:    make(chan struct{}),
		aggregationDone:    &sync.WaitGroup{},
		stopOutput:         make(chan struct{}),
		outputDone:         &sync.WaitGroup{},
	}, nil
}

// SetTestRunID sets the passed test run id.
func (out *Output) SetTestRunID(id string) {
	out.referenceID = id
}

// Start starts the Output, it starts the background goroutines
// for aggregating and flushing the collected metrics samples.
func (out *Output) Start() error {
	aggregationPeriod := out.config.AggregationPeriod.TimeDuration()
	// If enabled, start periodically aggregating the collected HTTP trails
	if aggregationPeriod > 0 {
		out.aggregationDone.Add(1)
		go func() {
			defer out.aggregationDone.Done()
			aggregationWaitPeriod := out.config.AggregationWaitPeriod.TimeDuration()
			aggregationTicker := time.NewTicker(aggregationPeriod)
			defer aggregationTicker.Stop()

			for {
				select {
				case <-out.stopSendingMetrics:
					return
				case <-aggregationTicker.C:
					out.aggregateHTTPTrails(aggregationWaitPeriod)
				case <-out.stopAggregation:
					out.aggregateHTTPTrails(0)
					out.flushHTTPTrails()
					return
				}
			}
		}()
	}

	if insightsOutput.Enabled(out.config) {
		testRunID, err := strconv.ParseInt(out.referenceID, 10, 64)
		if err != nil {
			return err
		}
		out.requestMetadatasCollector = insightsOutput.NewCollector(testRunID)

		insightsClientConfig := insights.NewDefaultClientConfigForTestRun(
			out.config.TracesHost.String,
			out.config.Token.String,
			testRunID,
		)
		insightsClient := insights.NewClient(insightsClientConfig)

		if err := insightsClient.Dial(context.Background()); err != nil {
			return err
		}

		out.insightsClient = insightsClient
		out.requestMetadatasFlusher = insightsOutput.NewFlusher(insightsClient, out.requestMetadatasCollector)
		out.runFlushRequestMetadatas()
	}

	out.outputDone.Add(1)
	go func() {
		defer out.outputDone.Done()
		pushTicker := time.NewTicker(out.config.MetricPushInterval.TimeDuration())
		defer pushTicker.Stop()
		for {
			select {
			case <-out.stopSendingMetrics:
				return
			default:
			}
			select {
			case <-out.stopOutput:
				out.pushMetrics()
				return
			case <-pushTicker.C:
				out.pushMetrics()
			}
		}
	}()

	return nil
}

// StopWithTestError gracefully stops all metric emission from the output: when
// all metric samples are emitted, it makes a cloud API call to finish the test
// run. If testErr was specified, it extracts the RunStatus from it.
func (out *Output) StopWithTestError(testErr error) error {
	out.logger.Debug("Stopping the cloud output...")
	close(out.stopAggregation)
	out.aggregationDone.Wait() // could be a no-op, if we have never started the aggregation
	out.logger.Debug("Aggregation stopped, stopping metric emission...")
	close(out.stopOutput)
	out.outputDone.Wait()
	out.logger.Debug("Metric emission stopped, calling cloud API...")
	if insightsOutput.Enabled(out.config) {
		if err := out.insightsClient.Close(); err != nil {
			out.logger.WithError(err).Error("Failed to close the insights client")
		}
	}
	return nil
}

// SetTestRunStopCallback receives the function that stops the engine on error
func (out *Output) SetTestRunStopCallback(stopFunc func(error)) {
	out.testStopFunc = stopFunc
}

// AddMetricSamples receives a set of metric samples. This method is never
// called concurrently, so it defers as much of the work as possible to the
// asynchronous goroutines initialized in Start().
func (out *Output) AddMetricSamples(sampleContainers []metrics.SampleContainer) {
	select {
	case <-out.stopSendingMetrics:
		return
	default:
	}

	if out.referenceID == "" {
		return
	}

	newSamples := []*Sample{}
	newHTTPTrails := []*httpext.Trail{}

	for _, sampleContainer := range sampleContainers {
		switch sc := sampleContainer.(type) {
		case *httpext.Trail:
			// Check if aggregation is enabled,
			if out.config.AggregationPeriod.Duration > 0 {
				newHTTPTrails = append(newHTTPTrails, sc)
			} else {
				newSamples = append(newSamples, NewSampleFromTrail(sc))
			}
		case *netext.NetTrail:
			// TODO: aggregate?
			values := map[string]float64{
				metrics.DataSentName:     float64(sc.BytesWritten),
				metrics.DataReceivedName: float64(sc.BytesRead),
			}

			if sc.FullIteration {
				values[metrics.IterationDurationName] = metrics.D(sc.EndTime.Sub(sc.StartTime))
				values[metrics.IterationsName] = 1
			}

			encodedTags, err := easyjson.Marshal(sc.GetTags())
			if err != nil {
				out.logger.WithError(err).Error("Encoding tags failed")
			}
			newSamples = append(newSamples, &Sample{
				Type:   DataTypeMap,
				Metric: "iter_li_all",
				Data: &SampleDataMap{
					Time:   toMicroSecond(sc.GetTime()),
					Tags:   encodedTags,
					Values: values,
				},
			})
		default:
			for _, sample := range sampleContainer.GetSamples() {
				encodedTags, err := easyjson.Marshal(sample.Tags)
				if err != nil {
					out.logger.WithError(err).Error("Encoding tags failed")
				}

				newSamples = append(newSamples, &Sample{
					Type:   DataTypeSingle,
					Metric: sample.Metric.Name,
					Data: &SampleDataSingle{
						Type:  sample.Metric.Type,
						Time:  toMicroSecond(sample.Time),
						Tags:  encodedTags,
						Value: sample.Value,
					},
				})
			}
		}
	}

	if len(newSamples) > 0 || len(newHTTPTrails) > 0 {
		out.bufferMutex.Lock()
		out.bufferSamples = append(out.bufferSamples, newSamples...)
		out.bufferHTTPTrails = append(out.bufferHTTPTrails, newHTTPTrails...)
		out.bufferMutex.Unlock()
	}

	if insightsOutput.Enabled(out.config) {
		out.requestMetadatasCollector.CollectRequestMetadatas(sampleContainers)
	}
}

//nolint:funlen,nestif,gocognit
func (out *Output) aggregateHTTPTrails(waitPeriod time.Duration) {
	out.bufferMutex.Lock()
	newHTTPTrails := out.bufferHTTPTrails
	out.bufferHTTPTrails = nil
	out.bufferMutex.Unlock()

	aggrPeriod := int64(out.config.AggregationPeriod.Duration)

	// Distribute all newly buffered HTTP trails into buckets and sub-buckets
	for _, trail := range newHTTPTrails {
		bucketID := trail.GetTime().UnixNano() / aggrPeriod

		// Get or create a time bucket for that trail period
		bucket, ok := out.aggrBuckets[bucketID]
		if !ok {
			bucket = aggregationBucket{}
			out.aggrBuckets[bucketID] = bucket
		}

		subBucket, ok := bucket[trail.Tags]
		if !ok {
			subBucket = make([]*httpext.Trail, 0, 100)
		}
		bucket[trail.Tags] = append(subBucket, trail)
	}

	// Which buckets are still new and we'll wait for trails to accumulate before aggregating
	bucketCutoffID := time.Now().Add(-waitPeriod).UnixNano() / aggrPeriod
	iqrRadius := out.config.AggregationOutlierIqrRadius.Float64
	iqrLowerCoef := out.config.AggregationOutlierIqrCoefLower.Float64
	iqrUpperCoef := out.config.AggregationOutlierIqrCoefUpper.Float64
	newSamples := []*Sample{}

	// Handle all aggregation buckets older than bucketCutoffID
	for bucketID, subBucket := range out.aggrBuckets {
		if bucketID > bucketCutoffID {
			continue
		}

		for tags, httpTrails := range subBucket {
			// start := time.Now() // this is in a combination with the log at the end
			trailCount := int64(len(httpTrails))
			if trailCount < out.config.AggregationMinSamples.Int64 {
				for _, trail := range httpTrails {
					newSamples = append(newSamples, NewSampleFromTrail(trail))
				}
				continue
			}
			encodedTags, err := easyjson.Marshal(tags)
			if err != nil {
				out.logger.WithError(err).Error("Encoding tags failed")
			}

			aggrData := &SampleDataAggregatedHTTPReqs{
				Time: toMicroSecond(time.Unix(0, bucketID*aggrPeriod+aggrPeriod/2)),
				Type: "aggregated_trend",
				Tags: encodedTags,
			}

			if out.config.AggregationSkipOutlierDetection.Bool {
				// Simply add up all HTTP trails, no outlier detection
				for _, trail := range httpTrails {
					aggrData.Add(trail)
				}
			} else {
				connDurations := make(durations, trailCount)
				reqDurations := make(durations, trailCount)
				for i, trail := range httpTrails {
					connDurations[i] = trail.ConnDuration
					reqDurations[i] = trail.Duration
				}

				var minConnDur, maxConnDur, minReqDur, maxReqDur time.Duration
				if trailCount < out.config.AggregationOutlierAlgoThreshold.Int64 {
					// Since there are fewer samples, we'll use the interpolation-enabled and
					// more precise sorting-based algorithm
					minConnDur, maxConnDur = connDurations.SortGetNormalBounds(iqrRadius, iqrLowerCoef, iqrUpperCoef, true)
					minReqDur, maxReqDur = reqDurations.SortGetNormalBounds(iqrRadius, iqrLowerCoef, iqrUpperCoef, true)
				} else {
					minConnDur, maxConnDur = connDurations.SelectGetNormalBounds(iqrRadius, iqrLowerCoef, iqrUpperCoef)
					minReqDur, maxReqDur = reqDurations.SelectGetNormalBounds(iqrRadius, iqrLowerCoef, iqrUpperCoef)
				}

				for _, trail := range httpTrails {
					if trail.ConnDuration < minConnDur ||
						trail.ConnDuration > maxConnDur ||
						trail.Duration < minReqDur ||
						trail.Duration > maxReqDur {
						// Seems like an outlier, add it as a standalone metric
						newSamples = append(newSamples, NewSampleFromTrail(trail))
					} else {
						// Aggregate the trail
						aggrData.Add(trail)
					}
				}
			}

			aggrData.CalcAverages()

			if aggrData.Count > 0 {
				newSamples = append(newSamples, &Sample{
					Type:   DataTypeAggregatedHTTPReqs,
					Metric: "http_req_li_all",
					Data:   aggrData,
				})
			}
		}
		delete(out.aggrBuckets, bucketID)
	}

	if len(newSamples) > 0 {
		out.bufferMutex.Lock()
		out.bufferSamples = append(out.bufferSamples, newSamples...)
		out.bufferMutex.Unlock()
	}
}

func (out *Output) flushHTTPTrails() {
	out.bufferMutex.Lock()
	defer out.bufferMutex.Unlock()

	newSamples := []*Sample{}
	for _, trail := range out.bufferHTTPTrails {
		newSamples = append(newSamples, NewSampleFromTrail(trail))
	}
	for _, bucket := range out.aggrBuckets {
		for _, subBucket := range bucket {
			for _, trail := range subBucket {
				newSamples = append(newSamples, NewSampleFromTrail(trail))
			}
		}
	}

	out.bufferHTTPTrails = nil
	out.aggrBuckets = map[int64]aggregationBucket{}
	out.bufferSamples = append(out.bufferSamples, newSamples...)
}

// shouldStopSendingMetrics returns true if the output should interrupt the metric flush.
//
// note: The actual test execution should continues,
// since for local k6 run tests the end-of-test summary (or any other outputs) will still work,
// but the cloud output doesn't send any more metrics.
// Instead, if cloudapi.Config.StopOnError is enabled
// the cloud output should stop the whole test run too.
// This logic should be handled by the caller.
func (out *Output) shouldStopSendingMetrics(err error) bool {
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

type pushJob struct {
	done    chan error
	samples []*Sample
}

// ceil(a/b)
func ceilDiv(a, b int) int {
	r := a / b
	if a%b != 0 {
		r++
	}
	return r
}

func (out *Output) pushMetrics() {
	out.bufferMutex.Lock()
	if len(out.bufferSamples) == 0 {
		out.bufferMutex.Unlock()
		return
	}
	buffer := out.bufferSamples
	out.bufferSamples = nil
	out.bufferMutex.Unlock()

	count := len(buffer)
	out.logger.WithFields(logrus.Fields{
		"samples": count,
	}).Debug("Pushing metrics to cloud")
	start := time.Now()

	numberOfPackages := ceilDiv(len(buffer), int(out.config.MaxMetricSamplesPerPackage.Int64))
	numberOfWorkers := int(out.config.MetricPushConcurrency.Int64)
	if numberOfWorkers > numberOfPackages {
		numberOfWorkers = numberOfPackages
	}

	ch := make(chan pushJob, numberOfPackages)
	for i := 0; i < numberOfWorkers; i++ {
		go func() {
			for job := range ch {
				err := out.client.PushMetric(out.referenceID, job.samples)
				job.done <- err
				if out.shouldStopSendingMetrics(err) {
					return
				}
			}
		}()
	}

	jobs := make([]pushJob, 0, numberOfPackages)

	for len(buffer) > 0 {
		size := len(buffer)
		if size > int(out.config.MaxMetricSamplesPerPackage.Int64) {
			size = int(out.config.MaxMetricSamplesPerPackage.Int64)
		}
		job := pushJob{done: make(chan error, 1), samples: buffer[:size]}
		ch <- job
		jobs = append(jobs, job)
		buffer = buffer[size:]
	}

	close(ch)

	for _, job := range jobs {
		err := <-job.done
		if err != nil {
			if out.shouldStopSendingMetrics(err) {
				out.logger.WithError(err).Warn("Stopped sending metrics to cloud due to an error")
				serr := errext.WithAbortReasonIfNone(
					errext.WithExitCodeIfNone(err, exitcodes.ExternalAbort),
					errext.AbortedByOutput,
				)
				if out.config.StopOnError.Bool {
					out.testStopFunc(serr)
				}
				close(out.stopSendingMetrics)
				break
			}
			out.logger.WithError(err).Warn("Failed to send metrics to cloud")
		}
	}
	out.logger.WithFields(logrus.Fields{
		"samples": count,
		"t":       time.Since(start),
	}).Debug("Pushing metrics to cloud finished")
}

func (out *Output) runFlushRequestMetadatas() {
	t := time.NewTicker(out.config.TracesPushInterval.TimeDuration())

	for i := int64(0); i < out.config.TracesPushConcurrency.Int64; i++ {
		out.outputDone.Add(1)
		go func() {
			defer out.outputDone.Done()
			defer t.Stop()

			for {
				select {
				case <-out.stopSendingMetrics:
					return
				default:
				}
				select {
				case <-out.stopOutput:
					out.flushRequestMetadatas()
					return
				case <-t.C:
					out.flushRequestMetadatas()
				}
			}
		}()
	}
}

func (out *Output) flushRequestMetadatas() {
	start := time.Now()

	err := out.requestMetadatasFlusher.Flush()
	if err != nil {
		out.logger.WithError(err).WithField("t", time.Since(start)).Error("Failed to push trace samples to the cloud")

		return
	}

	out.logger.WithField("t", time.Since(start)).Debug("Successfully flushed buffered trace samples to the cloud")
}

const expectedGzipRatio = 6 // based on test it is around 6.8, but we don't need to be that accurate
