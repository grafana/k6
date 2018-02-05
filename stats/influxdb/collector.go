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
	"sync"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
)

const (
	pushInterval = 1 * time.Second
)

var _ lib.Collector = &Collector{}

type Collector struct {
	Client    client.Client
	Config    Config
	BatchConf client.BatchPointsConfig

	buffer     []stats.Sample
	bufferLock sync.Mutex
}

func New(conf Config) (*Collector, error) {
	cl, err := MakeClient(conf)
	if err != nil {
		return nil, err
	}
	batchConf := MakeBatchConfig(conf)
	return &Collector{
		Client:    cl,
		Config:    conf,
		BatchConf: batchConf,
	}, nil
}

func (c *Collector) Init() error {
	// Try to create the database if it doesn't exist. Failure to do so is USUALLY harmless; it
	// usually means we're either a non-admin user to an existing DB or connecting over UDP.
	_, err := c.Client.Query(client.NewQuery("CREATE DATABASE "+c.BatchConf.Database, "", ""))
	if err != nil {
		log.WithError(err).Debug("InfluxDB: Couldn't create database; most likely harmless")
	}

	return nil
}

func (c *Collector) GetOptions() lib.CollectorOptions {
	return lib.CollectorOptions{}
}

func (c *Collector) Run(ctx context.Context) {
	log.Debug("InfluxDB: Running!")
	ticker := time.NewTicker(pushInterval)
	for {
		select {
		case <-ticker.C:
			c.commit()
		case <-ctx.Done():
			c.commit()
			return
		}
	}
}

func (c *Collector) Collect(samples []stats.Sample) {
	c.bufferLock.Lock()
	c.buffer = append(c.buffer, samples...)
	c.bufferLock.Unlock()
}

func (c *Collector) Link() string {
	return c.Config.Addr
}

func (c *Collector) commit() {
	c.bufferLock.Lock()
	samples := c.buffer
	c.buffer = nil
	c.bufferLock.Unlock()

	log.Debug("InfluxDB: Committing...")
	batch, err := client.NewBatchPoints(c.BatchConf)
	if err != nil {
		log.WithError(err).Error("InfluxDB: Couldn't make a batch")
		return
	}

	for _, sample := range samples {
		p, err := client.NewPoint(
			sample.Metric.Name,
			sample.Tags,
			map[string]interface{}{"value": sample.Value},
			sample.Time,
		)
		if err != nil {
			log.WithError(err).Error("InfluxDB: Couldn't make point from sample!")
			return
		}
		batch.AddPoint(p)
	}

	log.WithField("points", len(batch.Points())).Debug("InfluxDB: Writing...")
	startTime := time.Now()
	if err := c.Client.Write(batch); err != nil {
		log.WithError(err).Error("InfluxDB: Couldn't write stats")
	}
	t := time.Since(startTime)
	log.WithField("t", t).Debug("InfluxDB: Batch written!")
}
