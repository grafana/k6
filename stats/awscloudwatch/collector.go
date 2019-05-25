package awscloudwatch

import (
	"context"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"sync"
	"time"
)

type Collector struct {
	client          cloudWatchClient
	bufferedSamples []*sample
	bufferLock      sync.Mutex
}

func New() (*Collector, error) {
	return &Collector{}, nil
}

func (c *Collector) Init() error {
	panic("implement me")
}

func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(1 * time.Second))

	for {
		select {
		case <-ticker.C:
			c.reportMetrics()
		case <-ctx.Done():
			c.reportMetrics()
			return
		}
	}
}

func (c *Collector) reportMetrics() {
	c.bufferLock.Lock()
	_ = c.client.reportSamples(c.bufferedSamples)
	c.bufferedSamples = nil
	c.bufferLock.Unlock()
}

func (c *Collector) Collect(containers []stats.SampleContainer) {
	var samples []*sample
	for _, container := range containers {
		for _, s := range container.GetSamples() {
			samples = append(samples, &sample{
				Value:  s.Value,
				Time:   s.Time,
				Metric: s.Metric.Name,
			})
		}
	}

	c.bufferLock.Lock()
	c.bufferedSamples = append(c.bufferedSamples, samples...)
	c.bufferLock.Unlock()
}

func (c *Collector) Link() string {
	panic("implement me")
}

func (c *Collector) GetRequiredSystemTags() lib.TagSet {
	panic("implement me")
}

func (c *Collector) SetRunStatus(status lib.RunStatus) {
	panic("implement me")
}

type sample struct {
	Metric string
	Time   time.Time
	Value  float64
}

type cloudWatchClient interface {
	reportSamples(samples []*sample) error
}
