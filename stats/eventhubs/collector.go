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

package eventhubs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"

	eh "github.com/Azure/azure-event-hubs-go/v3"
)

// Collector sends result data to the Load Impact cloud service.
type Collector struct {
	config Config

	client eh.Hub

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
		client: *hub,
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
}

type TestEvent struct {
	Time     time.Time         `json:"time"`
	Value    float64           `json:"value"`
	Tags     *stats.SampleTags `json:"tags"`
	Name     string            `json:"name"`
	Contains string            `json:"contains"`
}

// Collect receives a set of samples. This method is never called concurrently, and only while
// the context for Run() is valid, but should defer as much work as possible to Run().
func (c *Collector) Collect(sampleContainers []stats.SampleContainer) {

	ctx := context.Background()

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			if &sample == nil {
				fmt.Println("sample is null")

				continue
			}

			data := TestEvent{
				Time:     sample.Time,
				Value:    sample.Value,
				Tags:     sample.Tags,
				Name:     sample.Metric.Name,
				Contains: sample.Metric.Contains.String(),
			}

			m, _ := json.Marshal(data)

			p := make(map[string]interface{})
			for key, value := range sample.Tags.CloneTags() {
				p[key] = value
			}

			event := eh.NewEvent(m)
			event.Properties = p

			c.client.Send(ctx, event)
		}
	}
}

func (c *Collector) pushMetrics() {
}

func (c *Collector) testFinished() {
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() stats.SystemTagSet {
	return stats.TagName | stats.TagMethod | stats.TagStatus | stats.TagError | stats.TagCheck | stats.TagGroup
}

// SetRunStatus Set run status
func (c *Collector) SetRunStatus(status lib.RunStatus) {
}
