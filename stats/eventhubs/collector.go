/*
*
* k6 - a next-generation load testing tool
* Copyright (C) 2020 Load Impact
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

package eventhubs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"

	eh "github.com/Azure/azure-event-hubs-go/v3"

	jsonc "github.com/loadimpact/k6/stats/json"
)

// Collector sends result data to the Load Impact cloud service.
type Collector struct {
	config Config

	client *eh.Hub

	ctx context.Context

	buffer     []stats.Sample
	bufferLock sync.Mutex

	logger logrus.FieldLogger
	opts   lib.Options
}

// Verify that Collector implements lib.Collector
var _ lib.Collector = &Collector{}

// New creates a new cloud collector
func New(logger logrus.FieldLogger, conf Config) (*Collector, error) {
	hub, _ := eh.NewHubFromConnectionString(conf.ConnectionString.String)
	return &Collector{
		config: conf,
		client: hub,
		buffer: make([]stats.Sample, 0),
		logger: logger,
	}, nil
}

// Init is called between the collector's creation and the call to Run().
// You should do any lengthy setup here rather than in New.
func (c *Collector) Init() error {
	return nil
}

// Link return a link that is shown to the user.
func (c *Collector) Link() string {
	return ""
}

// Run is called in a goroutine and starts the collector. Should commit samples to the backend
// at regular intervals and when the context is terminated.
func (c *Collector) Run(ctx context.Context) {
	c.ctx = ctx

	ticker := time.NewTicker(time.Duration(c.config.PushInterval.Duration))

	for {
		select {
		case <-ticker.C:
			c.pushMetrics()
		case <-ctx.Done():
			c.pushMetrics()
			c.finish()
			return
		}
	}
}

// HubEvent holds data to be sent to EventHubs
type HubEvent struct {
	Time        time.Time         `json:"time"`
	Value       float64           `json:"value"`
	Tags        *stats.SampleTags `json:"tags"`
	Name        string            `json:"name"`
	Contains    string            `json:"contains"`
	JSONContent *jsonc.Envelope   `json:"jsoncontent"`
}

// Collect receives a set of samples. This method is never called concurrently, and only while
// the context for Run() is valid, but should defer as much work as possible to Run().
func (c *Collector) Collect(sampleContainers []stats.SampleContainer) {
	c.bufferLock.Lock()
	for _, sampleContainer := range sampleContainers {
		c.buffer = append(c.buffer, sampleContainer.GetSamples()...)
	}
	c.bufferLock.Unlock()
}

func (c *Collector) pushMetrics() {
	if len(c.buffer) == 0 {
		return
	}
	c.bufferLock.Lock()
	buffer := c.buffer
	c.buffer = nil
	c.bufferLock.Unlock()

	events := make([]*eh.Event, 0)

	for _, sample := range buffer {
		env := jsonc.WrapSample(&sample)

		data := HubEvent{
			Time:        sample.Time,
			Value:       sample.Value,
			Tags:        sample.Tags,
			Name:        sample.Metric.Name,
			Contains:    sample.Metric.Contains.String(),
			JSONContent: env,
		}

		m, _ := json.Marshal(data)

		p := make(map[string]interface{})
		for key, value := range sample.Tags.CloneTags() {
			p[key] = value
		}

		event := eh.NewEvent(m)
		event.Properties = p

		events = append(events, event)
	}

	c.client.SendBatch(c.ctx, eh.NewEventBatchIterator(events...))
}

func (c *Collector) finish() {
	// Close when context is done

	fmt.Printf("done (%d)......\n", len(c.buffer))

	c.client.Close(c.ctx)
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() stats.SystemTagSet {
	return stats.TagName | stats.TagMethod | stats.TagStatus | stats.TagError | stats.TagCheck | stats.TagGroup
}

// SetRunStatus Set run status
func (c *Collector) SetRunStatus(status lib.RunStatus) {
}
