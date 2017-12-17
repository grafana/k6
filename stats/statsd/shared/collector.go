/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package statsd

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/ui"
	log "github.com/sirupsen/logrus"
)

const (
	pushInterval = 1 * time.Second
)

var _ lib.Collector = &Collector{}

// Tagger defines a type that can process tags
type Tagger interface {
	Process(string) func(map[string]string, string) []string
}

// Collector defines a collector struct
type Collector struct {
	Client  *statsd.Client
	Config  Config
	Logger  *log.Entry
	Type    ClientType
	Tagger  Tagger
	Summary map[string]*stats.Metric

	buffer     []*Sample
	bufferLock sync.Mutex
}

// Init sets up the collector
func (c *Collector) Init() error {
	c.Summary = make(map[string]*stats.Metric)
	return nil
}

// Link returns the address of the client
func (c *Collector) Link() string {
	return c.Config.Address()
}

// Run the collector
func (c *Collector) Run(ctx context.Context) {
	c.Logger.Debugf("%s: Running!", c.Type.String())
	ticker := time.NewTicker(pushInterval)
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

// Collect metrics
func (c *Collector) Collect(samples []stats.Sample) {
	var pointSamples []*Sample

	for _, sample := range samples {
		pointSamples = append(pointSamples, generateDataPoint(sample))
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
		Debugf("%s: Pushing metrics to server", c.Type.String())

	if err := c.commit(buffer); err != nil {
		c.Logger.
			WithError(err).
			Errorf("%s: Couldn't commit a batch", c.Type.String())
	}
}

func (c *Collector) finish() {
	if c.Type == DogStatsD {
		c.Logger.Debugf("%s: Generating summary event", c.Type.String())
		x := &bytes.Buffer{}
		ui.SummarizeMetrics(x, "", 1*time.Second, c.Summary)
		err := c.Client.SimpleEvent("[K6] Summary", x.String())
		if err != nil {
			c.Logger.
				WithError(err).
				Errorf("%s: Couldn't create event", c.Type.String())
		} else {
			err = c.Client.Flush()
			if err != nil {
				c.Logger.
					WithError(err).
					Errorf("%s: Couldn't send event", c.Type.String())
			} else {
				c.Logger.Debugf("%s: Summary event sent", c.Type.String())
			}
		}
	}
	// Close when context is done
	if err := c.Client.Close(); err != nil {
		c.Logger.Debugf("%s: Error closing the client, %+v", c.Type.String(), err)
	}
}

func (c *Collector) commit(data []*Sample) error {
	for _, entry := range data {
		c.dispatch(entry)
	}
	return c.Client.Flush()
}

func (c *Collector) dispatch(entry *Sample) {
	tagList := c.Tagger.Process(c.Config.Extra().TagWhitelist)(
		entry.Data.Tags,
		entry.Extra.Group,
	)

	c.Summary[entry.Metric] = entry.Extra.Raw

	switch entry.Type {
	case stats.Counter:
		_ = c.Client.Count(entry.Metric, int64(entry.Data.Value), tagList, 1)
	case stats.Trend:
		_ = c.Client.TimeInMilliseconds(entry.Metric, entry.Data.Value, tagList, 1)
	case stats.Gauge:
		_ = c.Client.Gauge(entry.Metric, entry.Data.Value, tagList, 1)
	case stats.Rate:
		if entry.Extra.Check != "" {
			label := "pass"
			if entry.Data.Value == 0 {
				label = "fail"
			}
			_ = c.Client.Count(fmt.Sprintf("check.%s.%s", entry.Extra.Check, label), 1, tagList, 1)
		} else {
			_ = c.Client.Count(entry.Metric, int64(entry.Data.Value), tagList, 1)
		}
	}
}
