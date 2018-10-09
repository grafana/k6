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
	"path/filepath"
	"sync"
	"time"

	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/pkg/errors"

	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
)

// TestName is the default Load Impact Cloud test name
const TestName = "k6 test"

// Collector sends result data to the Load Impact cloud service.
type Collector struct {
	config      Config
	referenceID string

	duration   int64
	thresholds map[string][]*stats.Threshold
	client     *Client

	anonymous bool
	runStatus lib.RunStatus

	bufferMutex      sync.Mutex
	bufferHTTPTrails []*netext.Trail
	bufferSamples    []*Sample

	opts lib.Options

	// TODO: optimize this
	//
	// Since the real-time metrics refactoring (https://github.com/loadimpact/k6/pull/678),
	// we should no longer have to handle metrics that have times long in the past. So instead of a
	// map, we can probably use a simple slice (or even an array!) as a ring buffer to store the
	// aggregation buckets. This should save us a some time, since it would make the lookups and WaitPeriod
	// checks basically O(1). And even if for some reason there are occasional metrics with past times that
	// don't fit in the chosen ring buffer size, we could just send them along to the buffer unaggregated
	aggrBuckets map[int64]aggregationBucket
}

// Verify that Collector implements lib.Collector
var _ lib.Collector = &Collector{}

// New creates a new cloud collector
func New(conf Config, src *lib.SourceData, opts lib.Options, version string) (*Collector, error) {
	if val, ok := opts.External["loadimpact"]; ok {
		if err := json.Unmarshal(val, &conf); err != nil {
			return nil, err
		}
	}

	if conf.AggregationPeriod.Duration > 0 && (opts.SystemTags["vu"] || opts.SystemTags["iter"]) {
		return nil, errors.New("Aggregation cannot be enabled if the 'vu' or 'iter' system tag is also enabled")
	}

	if !conf.Name.Valid || conf.Name.String == "" {
		conf.Name = null.StringFrom(filepath.Base(src.Filename))
	}
	if conf.Name.String == "-" {
		conf.Name = null.StringFrom(TestName)
	}

	thresholds := make(map[string][]*stats.Threshold)
	for name, t := range opts.Thresholds {
		thresholds[name] = append(thresholds[name], t.Thresholds...)
	}

	// Sum test duration from options. -1 for unknown duration.
	var duration int64 = -1
	if len(opts.Stages) > 0 {
		duration = sumStages(opts.Stages)
	} else if opts.Duration.Valid {
		duration = int64(time.Duration(opts.Duration.Duration).Seconds())
	}

	if duration == -1 {
		return nil, errors.New("Tests with unspecified duration are not allowed when using Load Impact Insights")
	}

	if !conf.Token.Valid && conf.DeprecatedToken.Valid {
		log.Warn("K6CLOUD_TOKEN is deprecated and will be removed. Use K6_CLOUD_TOKEN instead.")
		conf.Token = conf.DeprecatedToken
	}

	return &Collector{
		config:      conf,
		thresholds:  thresholds,
		client:      NewClient(conf.Token.String, conf.Host.String, version),
		anonymous:   !conf.Token.Valid,
		duration:    duration,
		opts:        opts,
		aggrBuckets: map[int64]aggregationBucket{},
	}, nil
}

func (c *Collector) Init() error {
	thresholds := make(map[string][]string)

	for name, t := range c.thresholds {
		for _, threshold := range t {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}

	testRun := &TestRun{
		Name:       c.config.Name.String,
		ProjectID:  c.config.ProjectID.Int64,
		VUsMax:     c.opts.VUsMax.Int64,
		Thresholds: thresholds,
		Duration:   c.duration,
	}

	response, err := c.client.CreateTestRun(testRun)
	if err != nil {
		return err
	}
	c.referenceID = response.ReferenceID

	if response.ConfigOverride != nil {
		log.WithFields(log.Fields{
			"override": response.ConfigOverride,
		}).Debug("Cloud: overriding config options")
		c.config = c.config.Apply(*response.ConfigOverride)
	}

	log.WithFields(log.Fields{
		"name":        c.config.Name,
		"projectId":   c.config.ProjectID,
		"duration":    c.duration,
		"referenceId": c.referenceID,
	}).Debug("Cloud: Initialized")
	return nil
}

func (c *Collector) Link() string {
	return URLForResults(c.referenceID, c.config)
}

func (c *Collector) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	// If enabled, start periodically aggregating the collected HTTP trails
	if c.config.AggregationPeriod.Duration > 0 {
		wg.Add(1)
		aggregationTicker := time.NewTicker(time.Duration(c.config.AggregationCalcInterval.Duration))

		go func() {
			for {
				select {
				case <-aggregationTicker.C:
					c.aggregateHTTPTrails(time.Duration(c.config.AggregationWaitPeriod.Duration))
				case <-ctx.Done():
					c.aggregateHTTPTrails(0)
					c.flushHTTPTrails()
					c.pushMetrics()
					wg.Done()
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
		case <-pushTicker.C:
			c.pushMetrics()
		case <-ctx.Done():
			c.pushMetrics()
			return
		}
	}
}

func (c *Collector) IsReady() bool {
	return true
}

func (c *Collector) Collect(sampleContainers []stats.SampleContainer) {
	if c.referenceID == "" {
		return
	}

	newSamples := []*Sample{}
	newHTTPTrails := []*netext.Trail{}

	for _, sampleContainer := range sampleContainers {
		switch sc := sampleContainer.(type) {
		case *netext.Trail:
			// Check if aggregation is enabled,
			if c.config.AggregationPeriod.Duration > 0 {
				newHTTPTrails = append(newHTTPTrails, sc)
			} else {
				newSamples = append(newSamples, NewSampleFromTrail(sc))
			}
		case *netext.NetTrail:
			//TODO: aggregate?
			values := map[string]float64{
				metrics.DataSent.Name:     float64(sc.BytesWritten),
				metrics.DataReceived.Name: float64(sc.BytesRead),
			}

			if sc.FullIteration {
				values[metrics.IterationDuration.Name] = stats.D(sc.EndTime.Sub(sc.StartTime))
			}

			newSamples = append(newSamples, &Sample{
				Type:   DataTypeMap,
				Metric: "iter_li_all",
				Data: &SampleDataMap{
					Time:   Timestamp(sc.GetTime()),
					Tags:   sc.GetTags(),
					Values: values,
				}})
		default:
			for _, sample := range sampleContainer.GetSamples() {
				newSamples = append(newSamples, &Sample{
					Type:   DataTypeSingle,
					Metric: sample.Metric.Name,
					Data: &SampleDataSingle{
						Type:  sample.Metric.Type,
						Time:  Timestamp(sample.Time),
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

func (c *Collector) aggregateHTTPTrails(waitPeriod time.Duration) {
	c.bufferMutex.Lock()
	newHTTPTrails := c.bufferHTTPTrails
	c.bufferHTTPTrails = nil
	c.bufferMutex.Unlock()

	aggrPeriod := int64(c.config.AggregationPeriod.Duration)

	// Distribute all newly buffered HTTP trails into buckets and sub-buckets
	for _, trail := range newHTTPTrails {
		trailTags := trail.GetTags()
		bucketID := trail.GetTime().UnixNano() / aggrPeriod

		// Get or create a time bucket for that trail period
		bucket, ok := c.aggrBuckets[bucketID]
		if !ok {
			bucket = aggregationBucket{}
			c.aggrBuckets[bucketID] = bucket
		}

		// Either use an existing subbucket key or use the trail tags as a new one
		subBucketKey := trailTags
		subBucket, ok := bucket[subBucketKey]
		if !ok {
			for sbTags, sb := range bucket {
				if trailTags.IsEqual(sbTags) {
					subBucketKey = sbTags
					subBucket = sb
				}
			}
		}
		bucket[subBucketKey] = append(subBucket, trail)
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

		for tags, httpTrails := range subBuckets {
			trailCount := int64(len(httpTrails))
			if trailCount < c.config.AggregationMinSamples.Int64 {
				for _, trail := range httpTrails {
					newSamples = append(newSamples, NewSampleFromTrail(trail))
				}
				continue
			}

			aggrData := &SampleDataAggregatedHTTPReqs{
				Time: Timestamp(time.Unix(0, bucketID*aggrPeriod+aggrPeriod/2)),
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
				log.WithFields(log.Fields{
					"http_samples": aggrData.Count,
				}).Debug("Aggregated HTTP metrics")
				newSamples = append(newSamples, &Sample{
					Type:   DataTypeAggregatedHTTPReqs,
					Metric: "http_req_li_all",
					Data:   aggrData,
				})
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
		for _, trails := range bucket {
			for _, trail := range trails {
				newSamples = append(newSamples, NewSampleFromTrail(trail))
			}
		}
	}

	c.bufferHTTPTrails = nil
	c.aggrBuckets = map[int64]aggregationBucket{}
	c.bufferSamples = append(c.bufferSamples, newSamples...)
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

	log.WithFields(log.Fields{
		"samples": len(buffer),
	}).Debug("Pushing metrics to cloud")

	err := c.client.PushMetric(c.referenceID, c.config.NoCompress.Bool, buffer)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Warn("Failed to send metrics to cloud")
	}
}

func (c *Collector) testFinished() {
	if c.referenceID == "" {
		return
	}

	testTainted := false
	thresholdResults := make(ThresholdResult)
	for name, thresholds := range c.thresholds {
		thresholdResults[name] = make(map[string]bool)
		for _, t := range thresholds {
			thresholdResults[name][t.Source] = t.Failed
			if t.Failed {
				testTainted = true
			}
		}
	}

	log.WithFields(log.Fields{
		"ref":     c.referenceID,
		"tainted": testTainted,
	}).Debug("Sending test finished")

	runStatus := lib.RunStatusFinished
	if c.runStatus != 0 {
		runStatus = c.runStatus
	}

	err := c.client.TestFinished(c.referenceID, thresholdResults, testTainted, runStatus)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Warn("Failed to send test finished to cloud")
	}
}

func sumStages(stages []lib.Stage) int64 {
	var total time.Duration
	for _, stage := range stages {
		total += time.Duration(stage.Duration.Duration)
	}

	return int64(total.Seconds())
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() lib.TagSet {
	return lib.GetTagSet("name", "method", "status", "error", "check", "group")
}

func (c *Collector) SetRunStatus(status lib.RunStatus) {
	c.runStatus = status
}
