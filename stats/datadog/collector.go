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

package datadog

import (
	"context"
	"sync"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
)

const (
	pushInterval = 1 * time.Second
)

var _ lib.Collector = &Collector{}

// Collector defines a collector struct
type Collector struct {
	Client *statsd.Client
	Config Config
	Logger *log.Entry

	buffer     []*Sample
	bufferLock sync.Mutex
}

// New initialises a collector
func New(conf Config) (*Collector, error) {
	cl, err := MakeClient(conf)
	if err != nil {
		return nil, err
	}
	return &Collector{
		Client: cl,
		Config: conf,
		Logger: log.WithField("type", "statsd"),
	}, nil
}

// Init sets up the collector
func (c *Collector) Init() error {
	return nil
}

// Link returns the address of the client
func (c *Collector) Link() string {
	return c.Config.Addr
}

// Run the collector
func (c *Collector) Run(ctx context.Context) {
	c.Logger.Debug("StatsD: Running!")
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
		Debug("Pushing metrics to cloud")

	if err := c.commit(buffer); err != nil {
		c.Logger.
			WithError(err).
			Error("StastD: Couldn't commit a batch")
	}
}

func (c *Collector) finish() {
	// Close when context is done
	if err := c.Client.Close(); err != nil {
		c.Logger.Debugf("StastD: Error closing the client, %+v", err)
	}
}

func (c *Collector) commit(data []*Sample) error {
	for _, entry := range data {
		switch entry.Type {
		case stats.Counter:
			c.Client.Count(entry.Metric, int64(entry.Data.Value), []string{}, 1)
			// log.Infof("counter -> %+v", fmt.Sprintf("%s|%v|%+v", entry.Metric, entry.Data.Value, entry.Data.Tags))
		case stats.Trend:
			c.Client.TimeInMilliseconds(entry.Metric, entry.Data.Value, []string{}, 1)
			// log.Infof("trend -> %+v", fmt.Sprintf("%s|%v|%+v", entry.Metric, entry.Data.Value, entry.Data.Tags))
		case stats.Gauge:
			c.Client.Gauge(entry.Metric, entry.Data.Value, []string{}, 1)
			// log.Infof("gauge -> %+v", fmt.Sprintf("%s|%v|%+v", entry.Metric, entry.Data.Value, entry.Data.Tags))
		case stats.Rate:
			// log.Infof("rate -> %+v", fmt.Sprintf("%s|%v|%+v", entry.Metric, entry.Data.Value, entry.Data.Tags))
		}

	}
	return c.Client.Flush()
}
