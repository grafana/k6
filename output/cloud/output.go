package cloud

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/output"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/metrics"
)

// TestName is the default Load Impact Cloud test name
const TestName = "k6 test"

// Output sends result data to the Load Impact cloud service.
type Output struct {
	config      cloudapi.Config
	referenceID string

	executionPlan []lib.ExecutionStep
	duration      int64 // in seconds
	thresholds    map[string][]*metrics.Threshold
	client        *MetricsClient

	runStatus lib.RunStatus

	bufferMutex      sync.Mutex
	bufferHTTPTrails []*httpext.Trail
	bufferSamples    []*Sample

	logger logrus.FieldLogger
	opts   lib.Options

	// TODO: optimize this
	//
	// Since the real-time metrics refactoring (https://github.com/k6io/k6/pull/678),
	// we should no longer have to handle metrics that have times long in the past. So instead of a
	// map, we can probably use a simple slice (or even an array!) as a ring buffer to store the
	// aggregation buckets. This should save us a some time, since it would make the lookups and WaitPeriod
	// checks basically O(1). And even if for some reason there are occasional metrics with past times that
	// don't fit in the chosen ring buffer size, we could just send them along to the buffer unaggregated
	aggrBuckets map[int64]map[[3]string]aggregationBucket

	stopSendingMetrics chan struct{}
	stopAggregation    chan struct{}
	aggregationDone    *sync.WaitGroup
	stopOutput         chan struct{}
	outputDone         *sync.WaitGroup
	engineStopFunc     func(error)
}

// Verify that Output implements the wanted interfaces
var _ interface {
	output.WithRunStatusUpdates
	output.WithThresholds
	output.WithTestRunStop
} = &Output{}

// New creates a new cloud output.
func New(params output.Params) (output.Output, error) {
	return newOutput(params)
}

// New creates a new cloud output.
func newOutput(params output.Params) (*Output, error) {
	conf, err := cloudapi.GetConsolidatedConfig(
		params.JSONConfig, params.Environment, params.ConfigArgument, params.ScriptOptions.External)
	if err != nil {
		return nil, err
	}

	if err := validateRequiredSystemTags(params.ScriptOptions.SystemTags); err != nil {
		return nil, err
	}

	logger := params.Logger.WithFields(logrus.Fields{"output": "cloud"})

	if conf.AggregationPeriod.Duration > 0 &&
		(params.ScriptOptions.SystemTags.Has(metrics.TagVU) || params.ScriptOptions.SystemTags.Has(metrics.TagIter)) {
		return nil, errors.New("aggregation cannot be enabled if the 'vu' or 'iter' system tag is also enabled")
	}

	if !conf.Name.Valid || conf.Name.String == "" {
		scriptPath := params.ScriptPath.String()
		if scriptPath == "" {
			// Script from stdin without a name, likely from stdin
			return nil, errors.New("script name not set, please specify K6_CLOUD_NAME or options.ext.loadimpact.name")
		}

		conf.Name = null.StringFrom(filepath.Base(scriptPath))
	}
	if conf.Name.String == "-" {
		conf.Name = null.StringFrom(TestName)
	}

	duration, testEnds := lib.GetEndOffset(params.ExecutionPlan)
	if !testEnds {
		return nil, errors.New("tests with unspecified duration are not allowed when outputting data to k6 cloud")
	}

	if !(conf.MetricPushConcurrency.Int64 > 0) {
		return nil, fmt.Errorf("metrics push concurrency must be a positive number but is %d",
			conf.MetricPushConcurrency.Int64)
	}

	if !(conf.MaxMetricSamplesPerPackage.Int64 > 0) {
		return nil, fmt.Errorf("metric samples per package must be a positive number but is %d",
			conf.MaxMetricSamplesPerPackage.Int64)
	}

	apiClient := cloudapi.NewClient(
		logger, conf.Token.String, conf.Host.String, consts.Version, conf.Timeout.TimeDuration())

	return &Output{
		config:        conf,
		client:        NewMetricsClient(apiClient, logger, conf.Host.String, conf.NoCompress.Bool),
		executionPlan: params.ExecutionPlan,
		duration:      int64(duration / time.Second),
		opts:          params.ScriptOptions,
		aggrBuckets:   map[int64]map[[3]string]aggregationBucket{},
		logger:        logger,

		stopSendingMetrics: make(chan struct{}),
		stopAggregation:    make(chan struct{}),
		aggregationDone:    &sync.WaitGroup{},
		stopOutput:         make(chan struct{}),
		outputDone:         &sync.WaitGroup{},
	}, nil
}

// validateRequiredSystemTags checks if all required tags are present.
func validateRequiredSystemTags(scriptTags *metrics.SystemTagSet) error {
	missingRequiredTags := []string{}
	requiredTags := metrics.TagName |
		metrics.TagMethod |
		metrics.TagStatus |
		metrics.TagError |
		metrics.TagCheck |
		metrics.TagGroup
	for _, tag := range metrics.SystemTagSetValues() {
		if requiredTags.Has(tag) && !scriptTags.Has(tag) {
			missingRequiredTags = append(missingRequiredTags, tag.String())
		}
	}
	if len(missingRequiredTags) > 0 {
		return fmt.Errorf(
			"the cloud output needs the following system tags enabled: %s",
			strings.Join(missingRequiredTags, ", "),
		)
	}
	return nil
}

// Start calls the k6 Cloud API to initialize the test run, and then starts the
// goroutine that would listen for metric samples and send them to the cloud.
func (out *Output) Start() error {
	if out.config.PushRefID.Valid {
		out.referenceID = out.config.PushRefID.String
		out.logger.WithField("referenceId", out.referenceID).Debug("directly pushing metrics without init")
		out.startBackgroundProcesses()
		return nil
	}

	thresholds := make(map[string][]string)

	for name, t := range out.thresholds {
		for _, threshold := range t {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}
	maxVUs := lib.GetMaxPossibleVUs(out.executionPlan)

	testRun := &cloudapi.TestRun{
		Name:       out.config.Name.String,
		ProjectID:  out.config.ProjectID.Int64,
		VUsMax:     int64(maxVUs),
		Thresholds: thresholds,
		Duration:   out.duration,
	}

	response, err := out.client.CreateTestRun(testRun)
	if err != nil {
		return err
	}
	out.referenceID = response.ReferenceID

	if response.ConfigOverride != nil {
		out.logger.WithFields(logrus.Fields{
			"override": response.ConfigOverride,
		}).Debug("overriding config options")
		out.config = out.config.Apply(*response.ConfigOverride)
	}

	out.startBackgroundProcesses()

	out.logger.WithFields(logrus.Fields{
		"name":        out.config.Name,
		"projectId":   out.config.ProjectID,
		"duration":    out.duration,
		"referenceId": out.referenceID,
	}).Debug("Started!")
	return nil
}

func (out *Output) startBackgroundProcesses() {
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
}

// Stop gracefully stops all metric emission from the output and when all metric
// samples are emitted, it sends an API to the cloud to finish the test run.
func (out *Output) Stop() error {
	out.logger.Debug("Stopping the cloud output...")
	close(out.stopAggregation)
	out.aggregationDone.Wait() // could be a no-op, if we have never started the aggregation
	out.logger.Debug("Aggregation stopped, stopping metric emission...")
	close(out.stopOutput)
	out.outputDone.Wait()
	out.logger.Debug("Metric emission stopped, calling cloud API...")
	err := out.testFinished()
	if err != nil {
		out.logger.WithFields(logrus.Fields{"error": err}).Warn("Failed to send test finished to the cloud")
	} else {
		out.logger.Debug("Cloud output successfully stopped!")
	}
	return err
}

// Description returns the URL with the test run results.
func (out *Output) Description() string {
	return fmt.Sprintf("cloud (%s)", cloudapi.URLForResults(out.referenceID, out.config))
}

// SetRunStatus receives the latest run status from the Engine.
func (out *Output) SetRunStatus(status lib.RunStatus) {
	out.runStatus = status
}

// SetThresholds receives the thresholds before the output is Start()-ed.
func (out *Output) SetThresholds(scriptThresholds map[string]metrics.Thresholds) {
	thresholds := make(map[string][]*metrics.Threshold)
	for name, t := range scriptThresholds {
		thresholds[name] = append(thresholds[name], t.Thresholds...)
	}
	out.thresholds = thresholds
}

// SetTestRunStopCallback receives the function that stops the engine on error
func (out *Output) SetTestRunStopCallback(stopFunc func(error)) {
	out.engineStopFunc = stopFunc
}

func useCloudTags(source *httpext.Trail) *httpext.Trail {
	name, nameExist := source.Tags.Get("name")
	url, urlExist := source.Tags.Get("url")
	if !nameExist || !urlExist || name == url {
		return source
	}

	newTags := source.Tags.CloneTags()
	newTags["url"] = name

	dest := new(httpext.Trail)
	*dest = *source
	dest.Tags = metrics.IntoSampleTags(&newTags)
	dest.Samples = nil

	return dest
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
			sc = useCloudTags(sc)
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

			newSamples = append(newSamples, &Sample{
				Type:   DataTypeMap,
				Metric: "iter_li_all",
				Data: &SampleDataMap{
					Time:   toMicroSecond(sc.GetTime()),
					Tags:   sc.GetTags(),
					Values: values,
				},
			})
		default:
			for _, sample := range sampleContainer.GetSamples() {
				newSamples = append(newSamples, &Sample{
					Type:   DataTypeSingle,
					Metric: sample.Metric.Name,
					Data: &SampleDataSingle{
						Type:  sample.Metric.Type,
						Time:  toMicroSecond(sample.Time),
						Tags:  sample.Tags,
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
}

//nolint:funlen,nestif,gocognit
func (out *Output) aggregateHTTPTrails(waitPeriod time.Duration) {
	out.bufferMutex.Lock()
	newHTTPTrails := out.bufferHTTPTrails
	out.bufferHTTPTrails = nil
	out.bufferMutex.Unlock()

	aggrPeriod := int64(out.config.AggregationPeriod.Duration)

	// Distribute all newly buffered HTTP trails into buckets and sub-buckets

	// this key is here specifically to not incur more allocations then necessary
	// if you change this code please run the benchmarks and add the results to the commit message
	var subBucketKey [3]string
	for _, trail := range newHTTPTrails {
		trailTags := trail.GetTags()
		bucketID := trail.GetTime().UnixNano() / aggrPeriod

		// Get or create a time bucket for that trail period
		bucket, ok := out.aggrBuckets[bucketID]
		if !ok {
			bucket = make(map[[3]string]aggregationBucket)
			out.aggrBuckets[bucketID] = bucket
		}
		subBucketKey[0], _ = trailTags.Get("name")
		subBucketKey[1], _ = trailTags.Get("group")
		subBucketKey[2], _ = trailTags.Get("status")

		subBucket, ok := bucket[subBucketKey]
		if !ok {
			subBucket = aggregationBucket{}
			bucket[subBucketKey] = subBucket
		}
		// Either use an existing subbucket key or use the trail tags as a new one
		subSubBucketKey := trailTags
		subSubBucket, ok := subBucket[subSubBucketKey]
		if !ok {
			for sbTags, sb := range subBucket {
				if trailTags.IsEqual(sbTags) {
					subSubBucketKey = sbTags
					subSubBucket = sb
					break
				}
			}
		}
		subBucket[subSubBucketKey] = append(subSubBucket, trail)
	}

	// Which buckets are still new and we'll wait for trails to accumulate before aggregating
	bucketCutoffID := time.Now().Add(-waitPeriod).UnixNano() / aggrPeriod
	iqrRadius := out.config.AggregationOutlierIqrRadius.Float64
	iqrLowerCoef := out.config.AggregationOutlierIqrCoefLower.Float64
	iqrUpperCoef := out.config.AggregationOutlierIqrCoefUpper.Float64
	newSamples := []*Sample{}

	// Handle all aggregation buckets older than bucketCutoffID
	for bucketID, subBuckets := range out.aggrBuckets {
		if bucketID > bucketCutoffID {
			continue
		}

		for _, subBucket := range subBuckets {
			for tags, httpTrails := range subBucket {
				// start := time.Now() // this is in a combination with the log at the end
				trailCount := int64(len(httpTrails))
				if trailCount < out.config.AggregationMinSamples.Int64 {
					for _, trail := range httpTrails {
						newSamples = append(newSamples, NewSampleFromTrail(trail))
					}
					continue
				}

				aggrData := &SampleDataAggregatedHTTPReqs{
					Time: toMicroSecond(time.Unix(0, bucketID*aggrPeriod+aggrPeriod/2)),
					Type: "aggregated_trend",
					Tags: tags,
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
					/*
						out.logger.WithFields(logrus.Fields{
							"http_samples": aggrData.Count,
							"ratio":        fmt.Sprintf("%.2f", float64(aggrData.Count)/float64(trailCount)),
							"t":            time.Since(start),
						}).Debug("Aggregated HTTP metrics")
					//*/
					newSamples = append(newSamples, &Sample{
						Type:   DataTypeAggregatedHTTPReqs,
						Metric: "http_req_li_all",
						Data:   aggrData,
					})
				}
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
			for _, trails := range subBucket {
				for _, trail := range trails {
					newSamples = append(newSamples, NewSampleFromTrail(trail))
				}
			}
		}
	}

	out.bufferHTTPTrails = nil
	out.aggrBuckets = map[int64]map[[3]string]aggregationBucket{}
	out.bufferSamples = append(out.bufferSamples, newSamples...)
}

func (out *Output) shouldStopSendingMetrics(err error) bool {
	if err == nil {
		return false
	}

	if errResp, ok := err.(cloudapi.ErrorResponse); ok && errResp.Response != nil {
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
				if out.config.StopOnError.Bool {
					out.engineStopFunc(err)
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

func (out *Output) testFinished() error {
	if out.referenceID == "" || out.config.PushRefID.Valid {
		return nil
	}

	testTainted := false
	thresholdResults := make(cloudapi.ThresholdResult)
	for name, thresholds := range out.thresholds {
		thresholdResults[name] = make(map[string]bool)
		for _, t := range thresholds {
			thresholdResults[name][t.Source] = t.LastFailed
			if t.LastFailed {
				testTainted = true
			}
		}
	}

	out.logger.WithFields(logrus.Fields{
		"ref":     out.referenceID,
		"tainted": testTainted,
	}).Debug("Sending test finished")

	runStatus := lib.RunStatusFinished
	if out.runStatus != lib.RunStatusQueued {
		runStatus = out.runStatus
	}

	return out.client.TestFinished(out.referenceID, thresholdResults, testTainted, runStatus)
}

const expectedGzipRatio = 6 // based on test it is around 6.8, but we don't need to be that accurate
