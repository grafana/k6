/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/sirupsen/logrus"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

var _ lib.Collector = &Collector{}

// Collector sends result data to statsd daemons with the ability to send to datadog as well
type Collector struct {
	Config Config
	Type   string
	// ProcessTags is called on a map of all tags for each metric and returns a slice representation
	// of those tags that should be sent. No tags are send in case of ProcessTags being null
	ProcessTags func(map[string]string) []string

	Logger     logrus.FieldLogger
	client     *statsd.Client
	startTime  time.Time
	buffer     []*Sample
	bufferLock sync.Mutex
}

// Init sets up the collector
func (c *Collector) Init() (err error) {
	c.Logger = c.Logger.WithField("type", c.Type)
	if address := c.Config.Addr.String; address == "" {
		err = fmt.Errorf(
			"connection string is invalid. Received: \"%+s\"",
			address,
		)
		c.Logger.Error(err)

		return err
	}

	c.client, err = statsd.NewBuffered(c.Config.Addr.String, int(c.Config.BufferSize.Int64))

	if err != nil {
		c.Logger.Errorf("Couldn't make buffered client, %s", err)
		return err
	}

	if namespace := c.Config.Namespace.String; namespace != "" {
		c.client.Namespace = namespace
	}

	return nil
}

// Link returns the address of the client
func (c *Collector) Link() string {
	return c.Config.Addr.String
}

// Run the collector
func (c *Collector) Run(ctx context.Context) {
	c.Logger.Debugf("%s: Running!", c.Type)
	ticker := time.NewTicker(time.Duration(c.Config.PushInterval.Duration))
	c.startTime = time.Now()

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

// GetRequiredSystemTags Return the required system sample tags for the specific collector
func (c *Collector) GetRequiredSystemTags() stats.SystemTagSet {
	return stats.SystemTagSet(0) // no tags are required
}

// SetRunStatus does nothing in statsd collector
func (c *Collector) SetRunStatus(status lib.RunStatus) {}

// Collect metrics
func (c *Collector) Collect(containers []stats.SampleContainer) {
	var pointSamples []*Sample

	for _, container := range containers {
		for _, sample := range container.GetSamples() {
			pointSamples = append(pointSamples, generateDataPoint(sample))
		}
	}

	if len(pointSamples) > 0 {
		c.bufferLock.Lock()
		c.buffer = append(c.buffer, pointSamples...)
		c.bufferLock.Unlock()
	}
}

func (c *Collector) pushMetrics() {
	c.bufferLock.Lock()
	if len(c.buffer) == 0 {
		c.bufferLock.Unlock()
		return
	}
	buffer := c.buffer
	c.buffer = nil
	c.bufferLock.Unlock()

	c.Logger.
		WithField("samples", len(buffer)).
		Debug("Pushing metrics to server")

	if err := c.commit(buffer); err != nil {
		c.Logger.
			WithError(err).
			Error("Couldn't commit a batch")
	}
}

func (c *Collector) finish() {
	// Close when context is done
	if err := c.client.Close(); err != nil {
		c.Logger.Warnf("Error closing the client, %+v", err)
	}
}

func (c *Collector) commit(data []*Sample) error {
	var errorCount int
	for _, entry := range data {
		if err := c.dispatch(entry); err != nil {
			// No need to return error if just one metric didn't go through
			c.Logger.WithError(err).Debugf("Error while sending metric %s", entry.Metric)
			errorCount++
		}
	}
	if errorCount != 0 {
		c.Logger.Warnf("Couldn't send %d out of %d metrics. Enable debug logging to see individual errors",
			errorCount, len(data))
	}
	return c.client.Flush()
}

func (c *Collector) dispatch(entry *Sample) error {
	var tagList []string
	if c.ProcessTags != nil {
		tagList = c.ProcessTags(entry.Tags)
	}

	switch entry.Type {
	case stats.Counter:
		return c.client.Count(entry.Metric, int64(entry.Value), tagList, 1)
	case stats.Trend:
		return c.client.TimeInMilliseconds(entry.Metric, entry.Value, tagList, 1)
	case stats.Gauge:
		return c.client.Gauge(entry.Metric, entry.Value, tagList, 1)
	case stats.Rate:
		if check := entry.Tags["check"]; check != "" {
			return c.client.Count(
				checkToString(check, entry.Value),
				1,
				tagList,
				1,
			)
		}
		return c.client.Count(entry.Metric, int64(entry.Value), tagList, 1)
	default:
		return fmt.Errorf("unsupported metric type %s", entry.Type)
	}
}

func checkToString(check string, value float64) string {
	label := "pass"
	if value == 0 {
		label = "fail"
	}
	return "check." + check + "." + label
}
