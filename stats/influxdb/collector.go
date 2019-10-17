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

package influxdb

import (
	"context"
	"errors"
	"sync"
	"time"

	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/sirupsen/logrus"
)

// Verify that Collector implements lib.Collector
var _ lib.Collector = &Collector{}

type Collector struct {
	Client    client.Client
	Config    Config
	BatchConf client.BatchPointsConfig

	buffer      []stats.Sample
	bufferLock  sync.Mutex
	wg          sync.WaitGroup
	semaphoreCh chan struct{}
}

func New(conf Config) (*Collector, error) {
	cl, err := MakeClient(conf)
	if err != nil {
		return nil, err
	}
	batchConf := MakeBatchConfig(conf)
	if conf.ConcurrentWrites.Int64 <= 0 {
		return nil, errors.New("influxdb's ConcurrentWrites must be a positive number")
	}
	return &Collector{
		Client:      cl,
		Config:      conf,
		BatchConf:   batchConf,
		semaphoreCh: make(chan struct{}, conf.ConcurrentWrites.Int64),
	}, nil
}

func (c *Collector) Init() error {
	// Try to create the database if it doesn't exist. Failure to do so is USUALLY harmless; it
	// usually means we're either a non-admin user to an existing DB or connecting over UDP.
	_, err := c.Client.Query(client.NewQuery("CREATE DATABASE "+c.BatchConf.Database, "", ""))
	if err != nil {
		logrus.WithError(err).Debug("InfluxDB: Couldn't create database; most likely harmless")
	}

	return nil
}

func (c *Collector) Run(ctx context.Context) {
	logrus.Debug("InfluxDB: Running!")
	ticker := time.NewTicker(time.Duration(c.Config.PushInterval.Duration))
	for {
		select {
		case <-ticker.C:
			c.wg.Add(1)
			go c.commit()
		case <-ctx.Done():
			c.wg.Add(1)
			go c.commit()
			c.wg.Wait()
			return
		}
	}
}

func (c *Collector) Collect(scs []stats.SampleContainer) {
	c.bufferLock.Lock()
	defer c.bufferLock.Unlock()
	for _, sc := range scs {
		c.buffer = append(c.buffer, sc.GetSamples()...)
	}
}

func (c *Collector) Link() string {
	return c.Config.Addr.String
}

func (c *Collector) commit() {
	defer c.wg.Done()
	c.bufferLock.Lock()
	samples := c.buffer
	c.buffer = nil
	c.bufferLock.Unlock()
	// let first get the data and then wait our turn
	c.semaphoreCh <- struct{}{}
	defer func() {
		<-c.semaphoreCh
	}()
	logrus.Debug("InfluxDB: Committing...")
	logrus.WithField("samples", len(samples)).Debug("InfluxDB: Writing...")

	batch, err := c.batchFromSamples(samples)
	if err != nil {
		return
	}

	logrus.WithField("points", len(batch.Points())).Debug("InfluxDB: Writing...")
	startTime := time.Now()
	if err := c.Client.Write(batch); err != nil {
		logrus.WithError(err).Error("InfluxDB: Couldn't write stats")
	}
	t := time.Since(startTime)
	logrus.WithField("t", t).Debug("InfluxDB: Batch written!")
}

func (c *Collector) extractTagsToValues(tags map[string]string, values map[string]interface{}) map[string]interface{} {
	for _, tag := range c.Config.TagsAsFields {
		if val, ok := tags[tag]; ok {
			values[tag] = val
			delete(tags, tag)
		}
	}
	return values
}

func (c *Collector) batchFromSamples(samples []stats.Sample) (client.BatchPoints, error) {
	batch, err := client.NewBatchPoints(c.BatchConf)
	if err != nil {
		logrus.WithError(err).Error("InfluxDB: Couldn't make a batch")
		return nil, err
	}

	type cacheItem struct {
		tags   map[string]string
		values map[string]interface{}
	}
	cache := map[*stats.SampleTags]cacheItem{}
	for _, sample := range samples {
		var tags map[string]string
		var values = make(map[string]interface{})
		if cached, ok := cache[sample.Tags]; ok {
			tags = cached.tags
			for k, v := range cached.values {
				values[k] = v
			}
		} else {
			tags = sample.Tags.CloneTags()
			c.extractTagsToValues(tags, values)
			cache[sample.Tags] = cacheItem{tags, values}
		}
		values["value"] = sample.Value
		p, err := client.NewPoint(
			sample.Metric.Name,
			tags,
			values,
			sample.Time,
		)
		if err != nil {
			logrus.WithError(err).Error("InfluxDB: Couldn't make point from sample!")
			return nil, err
		}
		batch.AddPoint(p)
	}

	return batch, err
}

// Format returns a string array of metrics in influx line-protocol
func (c *Collector) Format(samples []stats.Sample) ([]string, error) {
	var metrics []string
	batch, err := c.batchFromSamples(samples)

	if err != nil {
		return metrics, err
	}

	for _, point := range batch.Points() {
		metrics = append(metrics, point.String())
	}

	return metrics, nil
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() stats.SystemTagSet {
	return stats.SystemTagSet(0) // There are no required tags for this collector
}

// SetRunStatus does nothing in the InfluxDB collector
func (c *Collector) SetRunStatus(status lib.RunStatus) {}
