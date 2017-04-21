package cloud

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

// Collector sends results data to the Load Impact cloud service.
type Collector struct {
	referenceID string

	duration   int64
	thresholds map[string][]string
	client     *Client
}

func New(fname string, opts lib.Options) (*Collector, error) {
	referenceID := os.Getenv("K6CLOUD_REFERENCEID")
	token := os.Getenv("K6CLOUD_TOKEN")

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
	}

	return &Collector{
		referenceID: referenceID,
		thresholds:  thresholds,
		client:      NewClient(token),
		duration:    duration,
	}, nil
}

func (c *Collector) Init() {
	name := os.Getenv("K6CLOUD_NAME")
	if name == "" {
		name = "k6 test"
	}

	// TODO fix this and add proper error handling
	if c.referenceID == "" {
		response := c.client.CreateTestRun(name, c.thresholds, c.duration)
		if response != nil {
			c.referenceID = response.ReferenceID
		}
	}
}

func (c *Collector) String() string {
	return fmt.Sprintf("Load Impact (https://app.staging.loadimpact.com/k6/runs/%s)", c.referenceID)
}

func (c *Collector) Run(ctx context.Context) {
	t := time.Now()
	<-ctx.Done()
	s := time.Now()

	c.client.TestFinished(c.referenceID)

	log.Debug(fmt.Sprintf("http://localhost:5000/v1/metrics/%s/%d000/%d000\n", c.referenceID, t.Unix(), s.Unix()))
}

func (c *Collector) Collect(samples []stats.Sample) {

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

	if len(cloudSamples) > 0 && c.referenceID != "" {
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
