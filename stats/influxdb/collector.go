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
	"fmt"
	"net/url"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

const pushInterval = 1 * time.Second

type Collector struct {
	u         *url.URL
	client    client.Client
	batchConf client.BatchPointsConfig
	buffer    []stats.Sample
}

func New(s string, opts lib.Options) (*Collector, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	cl, batchConf, err := parseURL(u)
	if err != nil {
		return nil, err
	}

	return &Collector{
		u:         u,
		client:    cl,
		batchConf: batchConf,
	}, nil
}

func (c *Collector) String() string {
	return fmt.Sprintf("influxdb (%s)", c.u.Host)
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
	c.buffer = append(c.buffer, samples...)
}

func (c *Collector) commit() {
	samples := c.buffer
	c.buffer = nil

	log.Debug("InfluxDB: Committing...")
	batch, err := client.NewBatchPoints(c.batchConf)
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
	if err := c.client.Write(batch); err != nil {
		log.WithError(err).Error("InfluxDB: Couldn't write stats")
	}
	t := time.Since(startTime)
	log.WithField("t", t).Debug("InfluxDB: Batch written!")
}
