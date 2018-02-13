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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

const (
	TestName          = "k6 test"
	MetricPushinteral = 1 * time.Second
)

// Collector sends result data to the Load Impact cloud service.
type Collector struct {
	config      Config
	referenceID string

	duration   int64
	thresholds map[string][]*stats.Threshold
	client     *Client

	anonymous bool

	sampleBuffer []*Sample
	sampleMu     sync.Mutex
}

// New creates a new cloud collector
func New(conf Config, src *lib.SourceData, opts lib.Options, version string) (*Collector, error) {
	if val, ok := opts.External["loadimpact"]; ok {
		if err := mapstructure.Decode(val, &conf); err != nil {
			return nil, err
		}
	}

	if conf.Name == "" {
		conf.Name = filepath.Base(src.Filename)
	}
	if conf.Name == "" {
		conf.Name = TestName
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

	if conf.Token == "" && conf.DeprecatedToken != "" {
		log.Warn("K6CLOUD_TOKEN is deprecated and will be removed. Use K6_CLOUD_TOKEN instead.")
		conf.Token = conf.DeprecatedToken
	}

	return &Collector{
		config:     conf,
		thresholds: thresholds,
		client:     NewClient(conf.Token, conf.Host, version),
		anonymous:  conf.Token == "",
		duration:   duration,
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
		Name:       c.config.Name,
		ProjectID:  c.config.ProjectID,
		Thresholds: thresholds,
		Duration:   c.duration,
	}

	response, err := c.client.CreateTestRun(testRun)

	if err != nil {
		return err
	}
	c.referenceID = response.ReferenceID

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
	timer := time.NewTicker(MetricPushinteral)

	for {
		select {
		case <-timer.C:
			c.pushMetrics()
		case <-ctx.Done():
			c.pushMetrics()
			c.testFinished()
			return
		}
	}
}

func (c *Collector) IsReady() bool {
	return true
}

func (c *Collector) Collect(samples []stats.Sample) {
	if c.referenceID == "" {
		return
	}

	var cloudSamples []*Sample
	var httpJSON *Sample
	var iterationJSON *Sample
	for _, samp := range samples {

		name := samp.Metric.Name
		if name == "http_reqs" {
			httpJSON = &Sample{
				Type:   "Points",
				Metric: "http_req_li_all",
				Data: SampleData{
					Type:   samp.Metric.Type,
					Time:   samp.Time,
					Tags:   samp.Tags,
					Values: make(map[string]float64),
				},
			}
			httpJSON.Data.Values[name] = samp.Value
			cloudSamples = append(cloudSamples, httpJSON)
		} else if name == "data_sent" {
			iterationJSON = &Sample{
				Type:   "Points",
				Metric: "iter_li_all",
				Data: SampleData{
					Type:   samp.Metric.Type,
					Time:   samp.Time,
					Tags:   samp.Tags,
					Values: make(map[string]float64),
				},
			}
			iterationJSON.Data.Values[name] = samp.Value
			cloudSamples = append(cloudSamples, iterationJSON)
		} else if name == "data_received" || name == "iter_duration" {
			iterationJSON.Data.Values[name] = samp.Value
		} else if strings.HasPrefix(name, "http_req_") {
			httpJSON.Data.Values[name] = samp.Value
		} else {
			sampleJSON := &Sample{
				Type:   "Point",
				Metric: name,
				Data: SampleData{
					Type:  samp.Metric.Type,
					Time:  samp.Time,
					Value: samp.Value,
					Tags:  samp.Tags,
				},
			}
			cloudSamples = append(cloudSamples, sampleJSON)
		}
	}

	if len(cloudSamples) > 0 {
		c.sampleMu.Lock()
		c.sampleBuffer = append(c.sampleBuffer, cloudSamples...)
		c.sampleMu.Unlock()
	}
}

func (c *Collector) pushMetrics() {
	c.sampleMu.Lock()
	if len(c.sampleBuffer) == 0 {
		c.sampleMu.Unlock()
		return
	}
	buffer := c.sampleBuffer
	c.sampleBuffer = nil
	c.sampleMu.Unlock()

	log.WithFields(log.Fields{
		"samples": len(buffer),
	}).Debug("Pushing metrics to cloud")

	err := c.client.PushMetric(c.referenceID, c.config.NoCompress, buffer)
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

	err := c.client.TestFinished(c.referenceID, thresholdResults, testTainted)
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
