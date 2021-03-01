/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cloud

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mailru/easyjson"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/cloudapi"
	"github.com/loadimpact/k6/output"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/consts"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/stats"
)

// TestName is the default Load Impact Cloud test name
const TestName = "k6 test"

// Collector sends result data to the Load Impact cloud service.
type Collector struct {
	config      cloudapi.Config
	referenceID string

	executionPlan  []lib.ExecutionStep
	duration       int64 // in seconds
	thresholds     map[string][]*stats.Threshold
	client         *cloudapi.Client
	pushBufferPool sync.Pool

	runStatus lib.RunStatus

	bufferMutex      sync.Mutex
	bufferHTTPTrails []*httpext.Trail
	bufferSamples    []*Sample

	logger logrus.FieldLogger
	opts   lib.Options

	// TODO: optimize this
	//
	// Since the real-time metrics refactoring (https://github.com/loadimpact/k6/pull/678),
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
}

// Verify that Output implements the wanted interfaces
var _ interface {
	output.WithRunStatusUpdates
	output.WithThresholds
} = &Collector{}

// New creates a new cloud collector.
func New(params output.Params) (output.Output, error) {
	return newOutput(params)
}

// New creates a new cloud collector.
func newOutput(params output.Params) (*Collector, error) {
	conf, err := cloudapi.GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
	if err != nil {
		return nil, err
	}

	if err := validateRequiredSystemTags(params.ScriptOptions.SystemTags); err != nil {
		return nil, err
	}

	logger := params.Logger.WithFields(logrus.Fields{"output": "cloud"})

	if err := cloudapi.MergeFromExternal(params.ScriptOptions.External, &conf); err != nil {
		return nil, err
	}

	if conf.AggregationPeriod.Duration > 0 &&
		(params.ScriptOptions.SystemTags.Has(stats.TagVU) || params.ScriptOptions.SystemTags.Has(stats.TagIter)) {
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

	if !conf.Token.Valid && conf.DeprecatedToken.Valid {
		logger.Warn("K6CLOUD_TOKEN is deprecated and will be removed. Use K6_CLOUD_TOKEN instead.")
		conf.Token = conf.DeprecatedToken
	}

	if !(conf.MetricPushConcurrency.Int64 > 0) {
		return nil, errors.Errorf("metrics push concurrency must be a positive number but is %d",
			conf.MetricPushConcurrency.Int64)
	}

	if !(conf.MaxMetricSamplesPerPackage.Int64 > 0) {
		return nil, errors.Errorf("metric samples per package must be a positive number but is %d",
			conf.MaxMetricSamplesPerPackage.Int64)
	}

	return &Collector{
		config:        conf,
		client:        cloudapi.NewClient(logger, conf.Token.String, conf.Host.String, consts.Version),
		executionPlan: params.ExecutionPlan,
		duration:      int64(duration / time.Second),
		opts:          params.ScriptOptions,
		aggrBuckets:   map[int64]map[[3]string]aggregationBucket{},
		logger:        logger,
		pushBufferPool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
		stopSendingMetrics: make(chan struct{}),
		stopAggregation:    make(chan struct{}),
		aggregationDone:    &sync.WaitGroup{},
		stopOutput:         make(chan struct{}),
		outputDone:         &sync.WaitGroup{},
	}, nil
}

// validateRequiredSystemTags checks if all required tags are present.
func validateRequiredSystemTags(scriptTags *stats.SystemTagSet) error {
	missingRequiredTags := []string{}
	requiredTags := stats.TagName | stats.TagMethod | stats.TagStatus | stats.TagError | stats.TagCheck | stats.TagGroup
	for _, tag := range stats.SystemTagSetValues() {
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
func (c *Collector) Start() error {
	if c.config.PushRefID.Valid {
		c.referenceID = c.config.PushRefID.String
		c.logger.WithField("referenceId", c.referenceID).Debug("directly pushing metrics without init")
		return nil
	}

	thresholds := make(map[string][]string)

	for name, t := range c.thresholds {
		for _, threshold := range t {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}
	maxVUs := lib.GetMaxPossibleVUs(c.executionPlan)

	testRun := &cloudapi.TestRun{
		Name:       c.config.Name.String,
		ProjectID:  c.config.ProjectID.Int64,
		VUsMax:     int64(maxVUs),
		Thresholds: thresholds,
		Duration:   c.duration,
	}

	response, err := c.client.CreateTestRun(testRun)
	if err != nil {
		return err
	}
	c.referenceID = response.ReferenceID

	if response.ConfigOverride != nil {
		c.logger.WithFields(logrus.Fields{
			"override": response.ConfigOverride,
		}).Debug("overriding config options")
		c.config = c.config.Apply(*response.ConfigOverride)
	}

	c.startBackgroundProcesses()

	c.logger.WithFields(logrus.Fields{
		"name":        c.config.Name,
		"projectId":   c.config.ProjectID,
		"duration":    c.duration,
		"referenceId": c.referenceID,
	}).Debug("Started!")
	return nil
}

func (c *Collector) startBackgroundProcesses() {
	aggregationPeriod := time.Duration(c.config.AggregationPeriod.Duration)
	// If enabled, start periodically aggregating the collected HTTP trails
	if aggregationPeriod > 0 {
		c.aggregationDone.Add(1)
		go func() {
			defer c.aggregationDone.Done()
			aggregationWaitPeriod := time.Duration(c.config.AggregationWaitPeriod.Duration)
			aggregationTicker := time.NewTicker(aggregationPeriod)
			defer aggregationTicker.Stop()

			for {
				select {
				case <-c.stopSendingMetrics:
					return
				case <-aggregationTicker.C:
					c.aggregateHTTPTrails(aggregationWaitPeriod)
				case <-c.stopAggregation:
					c.aggregateHTTPTrails(0)
					c.flushHTTPTrails()
					return
				}
			}
		}()
	}

	c.outputDone.Add(1)
	go func() {
		defer c.outputDone.Done()
		pushTicker := time.NewTicker(time.Duration(c.config.MetricPushInterval.Duration))
		defer pushTicker.Stop()
		for {
			select {
			case <-c.stopSendingMetrics:
				return
			default:
			}
			select {
			case <-c.stopOutput:
				c.pushMetrics()
				return
			case <-pushTicker.C:
				c.pushMetrics()
			}
		}
	}()
}

// Stop gracefully stops all metric emission from the output and when all metric
// samples are emitted, it sends an API to the cloud to finish the test run.
func (c *Collector) Stop() error {
	c.logger.Debug("Stopping the cloud output...")
	close(c.stopAggregation)
	c.aggregationDone.Wait() // could be a no-op, if we have never started the aggregation
	c.logger.Debug("Aggregation stopped, stopping metric emission...")
	close(c.stopOutput)
	c.outputDone.Wait()
	c.logger.Debug("Metric emission stopped, calling cloud API...")
	err := c.testFinished()
	if err != nil {
		c.logger.WithFields(logrus.Fields{"error": err}).Warn("Failed to send test finished to the cloud")
	} else {
		c.logger.Debug("Cloud output successfully stopped!")
	}
	return err
}

// Description returns the URL with the test run results.
func (c *Collector) Description() string {
	return fmt.Sprintf("cloud (%s)", cloudapi.URLForResults(c.referenceID, c.config))
}

// SetRunStatus receives the latest run status from the Engine.
func (c *Collector) SetRunStatus(status lib.RunStatus) {
	c.runStatus = status
}

// SetThresholds receives the thresholds before the output is Start()-ed.
func (c *Collector) SetThresholds(scriptThresholds map[string]stats.Thresholds) {
	thresholds := make(map[string][]*stats.Threshold)
	for name, t := range scriptThresholds {
		thresholds[name] = append(thresholds[name], t.Thresholds...)
	}
	c.thresholds = thresholds
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
	dest.Tags = stats.IntoSampleTags(&newTags)
	dest.Samples = nil

	return dest
}

// AddMetricSamples receives a set of metric samples. This method is never
// called concurrently, so it defers as much of the work as possible to the
// asynchronous goroutines initialized in Start().
func (c *Collector) AddMetricSamples(sampleContainers []stats.SampleContainer) {
	select {
	case <-c.stopSendingMetrics:
		return
	default:
	}

	if c.referenceID == "" {
		return
	}

	newSamples := []*Sample{}
	newHTTPTrails := []*httpext.Trail{}

	for _, sampleContainer := range sampleContainers {
		switch sc := sampleContainer.(type) {
		case *httpext.Trail:
			sc = useCloudTags(sc)
			// Check if aggregation is enabled,
			if c.config.AggregationPeriod.Duration > 0 {
				newHTTPTrails = append(newHTTPTrails, sc)
			} else {
				newSamples = append(newSamples, NewSampleFromTrail(sc))
			}
		case *netext.NetTrail:
			// TODO: aggregate?
			values := map[string]float64{
				metrics.DataSent.Name:     float64(sc.BytesWritten),
				metrics.DataReceived.Name: float64(sc.BytesRead),
			}

			if sc.FullIteration {
				values[metrics.IterationDuration.Name] = stats.D(sc.EndTime.Sub(sc.StartTime))
				values[metrics.Iterations.Name] = 1
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
		c.bufferMutex.Lock()
		c.bufferSamples = append(c.bufferSamples, newSamples...)
		c.bufferHTTPTrails = append(c.bufferHTTPTrails, newHTTPTrails...)
		c.bufferMutex.Unlock()
	}
}

//nolint:funlen,nestif,gocognit
func (c *Collector) aggregateHTTPTrails(waitPeriod time.Duration) {
	c.bufferMutex.Lock()
	newHTTPTrails := c.bufferHTTPTrails
	c.bufferHTTPTrails = nil
	c.bufferMutex.Unlock()

	aggrPeriod := int64(c.config.AggregationPeriod.Duration)

	// Distribute all newly buffered HTTP trails into buckets and sub-buckets

	// this key is here specifically to not incur more allocations then necessary
	// if you change this code please run the benchmarks and add the results to the commit message
	var subBucketKey [3]string
	for _, trail := range newHTTPTrails {
		trailTags := trail.GetTags()
		bucketID := trail.GetTime().UnixNano() / aggrPeriod

		// Get or create a time bucket for that trail period
		bucket, ok := c.aggrBuckets[bucketID]
		if !ok {
			bucket = make(map[[3]string]aggregationBucket)
			c.aggrBuckets[bucketID] = bucket
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
	iqrRadius := c.config.AggregationOutlierIqrRadius.Float64
	iqrLowerCoef := c.config.AggregationOutlierIqrCoefLower.Float64
	iqrUpperCoef := c.config.AggregationOutlierIqrCoefUpper.Float64
	newSamples := []*Sample{}

	// Handle all aggregation buckets older than bucketCutoffID
	for bucketID, subBuckets := range c.aggrBuckets {
		if bucketID > bucketCutoffID {
			continue
		}

		for _, subBucket := range subBuckets {
			for tags, httpTrails := range subBucket {
				// start := time.Now() // this is in a combination with the log at the end
				trailCount := int64(len(httpTrails))
				if trailCount < c.config.AggregationMinSamples.Int64 {
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

				if c.config.AggregationSkipOutlierDetection.Bool {
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
					if trailCount < c.config.AggregationOutlierAlgoThreshold.Int64 {
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
						c.logger.WithFields(logrus.Fields{
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
		delete(c.aggrBuckets, bucketID)
	}

	if len(newSamples) > 0 {
		c.bufferMutex.Lock()
		c.bufferSamples = append(c.bufferSamples, newSamples...)
		c.bufferMutex.Unlock()
	}
}

func (c *Collector) flushHTTPTrails() {
	c.bufferMutex.Lock()
	defer c.bufferMutex.Unlock()

	newSamples := []*Sample{}
	for _, trail := range c.bufferHTTPTrails {
		newSamples = append(newSamples, NewSampleFromTrail(trail))
	}
	for _, bucket := range c.aggrBuckets {
		for _, subBucket := range bucket {
			for _, trails := range subBucket {
				for _, trail := range trails {
					newSamples = append(newSamples, NewSampleFromTrail(trail))
				}
			}
		}
	}

	c.bufferHTTPTrails = nil
	c.aggrBuckets = map[int64]map[[3]string]aggregationBucket{}
	c.bufferSamples = append(c.bufferSamples, newSamples...)
}

func (c *Collector) shouldStopSendingMetrics(err error) bool {
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

func (c *Collector) pushMetrics() {
	c.bufferMutex.Lock()
	if len(c.bufferSamples) == 0 {
		c.bufferMutex.Unlock()
		return
	}
	buffer := c.bufferSamples
	c.bufferSamples = nil
	c.bufferMutex.Unlock()

	count := len(buffer)
	c.logger.WithFields(logrus.Fields{
		"samples": count,
	}).Debug("Pushing metrics to cloud")
	start := time.Now()

	numberOfPackages := ceilDiv(len(buffer), int(c.config.MaxMetricSamplesPerPackage.Int64))
	numberOfWorkers := int(c.config.MetricPushConcurrency.Int64)
	if numberOfWorkers > numberOfPackages {
		numberOfWorkers = numberOfPackages
	}

	ch := make(chan pushJob, numberOfPackages)
	for i := 0; i < numberOfWorkers; i++ {
		go func() {
			for job := range ch {
				err := c.PushMetric(c.referenceID, c.config.NoCompress.Bool, job.samples)
				job.done <- err
				if c.shouldStopSendingMetrics(err) {
					return
				}
			}
		}()
	}

	jobs := make([]pushJob, 0, numberOfPackages)

	for len(buffer) > 0 {
		size := len(buffer)
		if size > int(c.config.MaxMetricSamplesPerPackage.Int64) {
			size = int(c.config.MaxMetricSamplesPerPackage.Int64)
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
			if c.shouldStopSendingMetrics(err) {
				c.logger.WithError(err).Warn("Stopped sending metrics to cloud due to an error")
				close(c.stopSendingMetrics)
				break
			}
			c.logger.WithError(err).Warn("Failed to send metrics to cloud")
		}
	}
	c.logger.WithFields(logrus.Fields{
		"samples": count,
		"t":       time.Since(start),
	}).Debug("Pushing metrics to cloud finished")
}

func (c *Collector) testFinished() error {
	if c.referenceID == "" || c.config.PushRefID.Valid {
		return nil
	}

	testTainted := false
	thresholdResults := make(cloudapi.ThresholdResult)
	for name, thresholds := range c.thresholds {
		thresholdResults[name] = make(map[string]bool)
		for _, t := range thresholds {
			thresholdResults[name][t.Source] = t.LastFailed
			if t.LastFailed {
				testTainted = true
			}
		}
	}

	c.logger.WithFields(logrus.Fields{
		"ref":     c.referenceID,
		"tainted": testTainted,
	}).Debug("Sending test finished")

	runStatus := lib.RunStatusFinished
	if c.runStatus != lib.RunStatusQueued {
		runStatus = c.runStatus
	}

	return c.client.TestFinished(c.referenceID, thresholdResults, testTainted, runStatus)
}

const expectedGzipRatio = 6 // based on test it is around 6.8, but we don't need to be that accurate

// PushMetric pushes the provided metric samples for the given referenceID
func (c *Collector) PushMetric(referenceID string, noCompress bool, s []*Sample) error {
	start := time.Now()
	url := fmt.Sprintf("%s/v1/metrics/%s", c.config.Host.String, referenceID)

	jsonStart := time.Now()
	b, err := easyjson.Marshal(samples(s))
	if err != nil {
		return err
	}
	jsonTime := time.Since(jsonStart)

	// TODO: change the context, maybe to one with a timeout
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Payload-Sample-Count", strconv.Itoa(len(s)))
	var additionalFields logrus.Fields

	if !noCompress {
		buf := c.pushBufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer c.pushBufferPool.Put(buf)
		unzippedSize := len(b)
		buf.Grow(unzippedSize / expectedGzipRatio)
		gzipStart := time.Now()
		{
			g, _ := gzip.NewWriterLevel(buf, gzip.BestSpeed)
			if _, err = g.Write(b); err != nil {
				return err
			}
			if err = g.Close(); err != nil {
				return err
			}
		}
		gzipTime := time.Since(gzipStart)

		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("X-Payload-Byte-Count", strconv.Itoa(unzippedSize))

		additionalFields = logrus.Fields{
			"unzipped_size":  unzippedSize,
			"gzip_t":         gzipTime,
			"content_length": buf.Len(),
		}

		b = buf.Bytes()
	}

	req.Header.Set("Content-Length", strconv.Itoa(len(b)))
	req.Body = ioutil.NopCloser(bytes.NewReader(b))
	req.GetBody = func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	}

	err = c.client.Do(req, nil)

	c.logger.WithFields(logrus.Fields{
		"t":         time.Since(start),
		"json_t":    jsonTime,
		"part_size": len(s),
	}).WithFields(additionalFields).Debug("Pushed part to cloud")

	return err
}
