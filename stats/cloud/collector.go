package cloud

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"

	"github.com/mitchellh/mapstructure"
)

type loadimpactConfig struct {
	ProjectId int    `mapstructure:"project_id"`
	Name      string `mapstructure:"name"`
}

// Collector sends results data to the Load Impact cloud service.
type Collector struct {
	referenceID string

	name       string
	project_id int

	duration   int64
	thresholds map[string][]string
	client     *Client
}

// New creates a new cloud collector
func New(fname string, src *lib.SourceData, opts lib.Options) (*Collector, error) {
	token := os.Getenv("K6CLOUD_TOKEN")

	var extConfig loadimpactConfig
	if val, ok := opts.External["loadimpact"]; ok == true {
		err := mapstructure.Decode(val, &extConfig)
		if err != nil {
			// For now we ignore if loadimpact section is malformed
		}
	}

	thresholds := make(map[string][]string)

	for name, t := range opts.Thresholds {
		for _, threshold := range t.Thresholds {
			thresholds[name] = append(thresholds[name], threshold.Source)
		}
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
		client:     NewClient(token),
		duration:   duration,
	}, nil
}

func (c *Collector) Init() {
	testRun := &TestRun{
		Name:       c.name,
		Thresholds: c.thresholds,
		Duration:   c.duration,
		ProjectID:  c.project_id,
	}

	// TODO fix this and add proper error handling
	response := c.client.CreateTestRun(testRun)
	if response != nil {
		c.referenceID = response.ReferenceID
	} else {
		log.Warn("Failed to create test in Load Impact cloud")
	}

	log.WithFields(log.Fields{
		"name":        c.name,
		"projectId":   c.project_id,
		"duration":    c.duration,
		"referenceId": c.referenceID,
	}).Debug("Cloud collector init")
}

func (c *Collector) String() string {
	return fmt.Sprintf("Load Impact (https://app.staging.loadimpact.com/k6/runs/%s)", c.referenceID)
}

func (c *Collector) Run(ctx context.Context) {
	<-ctx.Done()

	if c.referenceID != "" {
		c.client.TestFinished(c.referenceID)
	}
}

func (c *Collector) Collect(samples []stats.Sample) {
	if c.referenceID == "" {
		return
	}

	var cloudSamples []*Sample
	for _, sample := range samples {
		sampleJSON := &Sample{
			Type:   "Point",
			Metric: sample.Metric.Name,
			Data: SampleData{
				Type:  sample.Metric.Type,
				Time:  sample.Time,
				Value: sample.Value,
				Tags:  sample.Tags,
			},
		}
		cloudSamples = append(cloudSamples, sampleJSON)
	}

	if len(cloudSamples) > 0 {
		c.client.PushMetric(c.referenceID, cloudSamples)
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
