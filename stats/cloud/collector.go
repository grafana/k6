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
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
)

// TestName is the default Load Impact Cloud test name
const TestName = "k6 test"

// Collector sends result data to the Load Impact cloud service.
type Collector struct {
	config      Config
	referenceID string

	executionPlan []lib.ExecutionStep
	duration      int64 // in seconds
	thresholds    map[string][]*stats.Threshold
	client        *Client

	anonymous bool
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

	stopSendingMetricsCh chan struct{}
}

// Verify that Collector implements lib.Collector
var _ lib.Collector = &Collector{}

// MergeFromExternal merges three fields from json in a loadimact key of the provided external map
func MergeFromExternal(external map[string]json.RawMessage, conf *Config) error {
	if val, ok := external["loadimpact"]; ok {
		// TODO: Important! Separate configs and fix the whole 2 configs mess!
		tmpConfig := Config{}
		if err := json.Unmarshal(val, &tmpConfig); err != nil {
			return err
		}
		// Only take out the ProjectID, Name and Token from the options.ext.loadimpact map:
		if tmpConfig.ProjectID.Valid {
			conf.ProjectID = tmpConfig.ProjectID
		}
		if tmpConfig.Name.Valid {
			conf.Name = tmpConfig.Name
		}
		if tmpConfig.Token.Valid {
			conf.Token = tmpConfig.Token
		}
	}
	return nil
}

// New creates a new cloud collector
func New(
	logger logrus.FieldLogger,
	conf Config, src *loader.SourceData, opts lib.Options, executionPlan []lib.ExecutionStep, version string,
) (*Collector, error) {
	if err := MergeFromExternal(opts.External, &conf); err != nil {
		return nil, err
	}

	if conf.AggregationPeriod.Duration > 0 && (opts.SystemTags.Has(stats.TagVU) || opts.SystemTags.Has(stats.TagIter)) {
		return nil, errors.New("Aggregation cannot be enabled if the 'vu' or 'iter' system tag is also enabled")
	}

	if !conf.Name.Valid || conf.Name.String == "" {
		conf.Name = null.StringFrom(filepath.Base(src.URL.String()))
	}
	if conf.Name.String == "-" {
		conf.Name = null.StringFrom(TestName)
	}

	thresholds := make(map[string][]*stats.Threshold)
	for name, t := range opts.Thresholds {
		thresholds[name] = append(thresholds[name], t.Thresholds...)
	}

	duration, testEnds := lib.GetEndOffset(executionPlan)
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
		config:               conf,
		thresholds:           thresholds,
		client:               NewClient(logger, conf.Token.String, conf.Host.String, version),
		anonymous:            !conf.Token.Valid,
		executionPlan:        executionPlan,
		duration:             int64(duration / time.Second),
		opts:                 opts,
		aggrBuckets:          map[int64]map[[3]string]aggregationBucket{},
		stopSendingMetricsCh: make(chan struct{}),
		logger:               logger,
	}, nil
}

// Init is called between the collector's creation and the call to Run().
// You should do any lengthy setup here rather than in New.
func (c *Collector) Init() error {
	if c.config.PushRefID.Valid {
		c.referenceID = c.config.PushRefID.String
		c.logger.WithField("referenceId", c.referenceID).Debug("Cloud: directly pushing metrics without init")
		return nil
	}

	thresholds := make(map[string][]string)

	for name, t := range c.thresholds {
		for _, threshold := range t {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}
	maxVUs := lib.GetMaxPossibleVUs(c.executionPlan)

	testRun := &TestRun{
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
		}).Debug("Cloud: overriding config options")
		c.config = c.config.Apply(*response.ConfigOverride)
	}

	c.logger.WithFields(logrus.Fields{
		"name":        c.config.Name,
		"projectId":   c.config.ProjectID,
		"duration":    c.duration,
		"referenceId": c.referenceID,
	}).Debug("Cloud: Initialized")
	return nil
}

// Link return a link that is shown to the user.
func (c *Collector) Link() string {
	return URLForResults(c.referenceID, c.config)
}

// Run is called in a goroutine and starts the collector. Should commit samples to the backend
// at regular intervals and when the context is terminated.
func (c *Collector) Run(ctx context.Context) {
	wg := sync.WaitGroup{}
	quit := ctx.Done()
	aggregationPeriod := time.Duration(c.config.AggregationPeriod.Duration)
	// If enabled, start periodically aggregating the collected HTTP trails
	if aggregationPeriod > 0 {
		wg.Add(1)
		aggregationTicker := time.NewTicker(aggregationPeriod)
		aggregationWaitPeriod := time.Duration(c.config.AggregationWaitPeriod.Duration)
		signalQuit := make(chan struct{})
		quit = signalQuit

		go func() {
			defer wg.Done()
			for {
				select {
				case <-c.stopSendingMetricsCh:
					return
				case <-aggregationTicker.C:
					c.aggregateHTTPTrails(aggregationWaitPeriod)
				case <-ctx.Done():
					c.aggregateHTTPTrails(0)
					c.flushHTTPTrails()
					close(signalQuit)
					return
				}
			}
		}()
	}

	defer func() {
		wg.Wait()
		c.testFinished()
	}()

	pushTicker := time.NewTicker(time.Duration(c.config.MetricPushInterval.Duration))
	for {
		select {
		case <-c.stopSendingMetricsCh:
			return
		default:
		}
		select {
		case <-quit:
			c.pushMetrics()
			return
		case <-pushTicker.C:
			c.pushMetrics()
		}
	}
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

// Collect receives a set of samples. This method is never called concurrently, and only while
// the context for Run() is valid, but should defer as much work as possible to Run().
func (c *Collector) Collect(sampleContainers []stats.SampleContainer) {
	select {
	case <-c.stopSendingMetricsCh:
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

	if errResp, ok := err.(ErrorResponse); ok && errResp.Response != nil {
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
				err := c.client.PushMetric(c.referenceID, c.config.NoCompress.Bool, job.samples)
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
				close(c.stopSendingMetricsCh)
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

func (c *Collector) testFinished() {
	if c.referenceID == "" || c.config.PushRefID.Valid {
		return
	}

	testTainted := false
	thresholdResults := make(ThresholdResult)
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

	err := c.client.TestFinished(c.referenceID, thresholdResults, testTainted, runStatus)
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"error": err,
		}).Warn("Failed to send test finished to cloud")
	}
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() stats.SystemTagSet {
	return stats.TagName | stats.TagMethod | stats.TagStatus | stats.TagError | stats.TagCheck | stats.TagGroup
}

// SetRunStatus Set run status
func (c *Collector) SetRunStatus(status lib.RunStatus) {
	c.runStatus = status
}
