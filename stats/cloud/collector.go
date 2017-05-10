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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/mitchellh/mapstructure"
)

type loadimpactConfig struct {
	ProjectId int    `mapstructure:"project_id"`
	Name      string `mapstructure:"name"`
}

// Collector sends result data to the Load Impact cloud service.
type Collector struct {
	referenceID string
	initErr     error // Possible error from init call to cloud API

	name       string
	project_id int

	duration   int64
	thresholds map[string][]*stats.Threshold
	client     *Client
}

// New creates a new cloud collector
func New(fname string, src *lib.SourceData, opts lib.Options, version string) (*Collector, error) {
	token := os.Getenv("K6CLOUD_TOKEN")

	var extConfig loadimpactConfig
	if val, ok := opts.External["loadimpact"]; ok {
		err := mapstructure.Decode(val, &extConfig)
		if err != nil {
			log.Warn("Malformed loadimpact settings in script options")
		}
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
		// Parse duration if no stages found
		dur, err := time.ParseDuration(opts.Duration.String)
		// ignore error and keep default -1 value
		if err == nil {
			duration = int64(dur.Seconds())
		}
	}

	return &Collector{
		name:       getName(src, extConfig),
		project_id: getProjectId(extConfig),
		thresholds: thresholds,
		client:     NewClient(token, "", version),
		duration:   duration,
	}, nil
}

func (c *Collector) Init() {
	thresholds := make(map[string][]string)

	for name, t := range c.thresholds {
		for _, threshold := range t {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
	}

	testRun := &TestRun{
		Name:       c.name,
		Thresholds: thresholds,
		Duration:   c.duration,
		ProjectID:  c.project_id,
	}

	response, err := c.client.CreateTestRun(testRun)

	if err != nil {
		c.initErr = err
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Cloud collector failed to init")
		return
	}
	c.referenceID = response.ReferenceID

	log.WithFields(log.Fields{
		"name":        c.name,
		"projectId":   c.project_id,
		"duration":    c.duration,
		"referenceId": c.referenceID,
	}).Debug("Cloud collector init successful")
}

func (c *Collector) String() string {
	if c.initErr == nil {
		return fmt.Sprintf("Load Impact (https://app.loadimpact.com/k6/runs/%s)", c.referenceID)
	}

	switch c.initErr {
	case ErrNotAuthorized:
	case ErrNotAuthorized:
		return c.initErr.Error()
	}
	return fmt.Sprintf("Failed to create test in Load Impact cloud")
}

func (c *Collector) Run(ctx context.Context) {
	<-ctx.Done()

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

	if c.referenceID != "" {
		err := c.client.TestFinished(c.referenceID, thresholdResults, testTainted)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Warn("Failed to send test finished to cloud")
		}
	}
}

func (c *Collector) Collect(samples []stats.Sample) {
	if c.referenceID == "" {
		return
	}

	var cloudSamples []*sample
	for _, samp := range samples {
		sampleJSON := &sample{
			Type:   "Point",
			Metric: samp.Metric.Name,
			Data: sampleData{
				Type:  samp.Metric.Type,
				Time:  samp.Time,
				Value: samp.Value,
				Tags:  samp.Tags,
			},
		}
		cloudSamples = append(cloudSamples, sampleJSON)
	}

	if len(cloudSamples) > 0 {
		err := c.client.PushMetric(c.referenceID, cloudSamples)
		if err != nil {
			log.WithFields(log.Fields{
				"error":   err,
				"samples": cloudSamples,
			}).Warn("Failed to send metrics to cloud")
		}
	}
}

func sumStages(stages []lib.Stage) int64 {
	var total time.Duration
	for _, stage := range stages {
		total += stage.Duration
	}

	return int64(total.Seconds())
}

func getProjectId(extConfig loadimpactConfig) int {
	env := os.Getenv("K6CLOUD_PROJECTID")
	if env != "" {
		id, err := strconv.Atoi(env)
		if err == nil && id > 0 {
			return id
		}
	}

	if extConfig.ProjectId > 0 {
		return extConfig.ProjectId
	}

	return 0
}

func getName(src *lib.SourceData, extConfig loadimpactConfig) string {
	envName := os.Getenv("K6CLOUD_NAME")
	if envName != "" {
		return envName
	}

	if extConfig.Name != "" {
		return extConfig.Name
	}

	if src.Filename != "" {
		name := filepath.Base(src.Filename)
		if name != "" || name != "." {
			return name
		}
	}

	return "k6 test"
}
