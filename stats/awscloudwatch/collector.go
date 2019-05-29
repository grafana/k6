package awscloudwatch

import (
	"context"
	"sync"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
)

// Collector collects metrics and send them to CloudWatch
type Collector struct {
	client          cloudWatchClient
	bufferedSamples []*sample
	bufferLock      sync.Mutex
}

// New returns a new Collector
func New(clientFactory func() (cloudWatchClient, error)) (*Collector, error) {
	client, err := clientFactory()
	if err != nil {
		return nil, err
	}

	return &Collector{client: client}, nil
}

// Init does nothing, as the client is injected in New constructor
func (c *Collector) Init() error {
	return nil
}

// Run send collected metrics every second and when context is done
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)

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
	err := c.client.reportSamples(c.bufferedSamples)
	if err != nil {
		log.WithError(err).Error("Sending samples to CloudWatch")
	}
	c.bufferedSamples = nil
	c.bufferLock.Unlock()
}

// Collect collects samples from containers
func (c *Collector) Collect(containers []stats.SampleContainer) {
	var samples []*sample
	for _, container := range containers {
		for _, s := range container.GetSamples() {
			samples = append(samples, &sample{
				Value:  s.Value,
				Time:   s.Time,
				Metric: s.Metric.Name,
				Tags:   s.Tags.CloneTags(),
			})
		}
	}

	c.bufferLock.Lock()
	c.bufferedSamples = append(c.bufferedSamples, samples...)
	c.bufferLock.Unlock()
}

// Link returns link to AWS client address(i.e., regional monitoring)
func (c *Collector) Link() string {
	return c.client.address()
}

// GetRequiredSystemTags returns a tag set without requirements
func (c *Collector) GetRequiredSystemTags() lib.TagSet {
	return lib.TagSet{}
}

// SetRunStatus does nothing
func (c *Collector) SetRunStatus(status lib.RunStatus) {}

type sample struct {
	Metric string
	Time   time.Time
	Value  float64
	Tags   map[string]string
}

type cloudWatchClient interface {
	reportSamples(samples []*sample) error
	address() string
}
