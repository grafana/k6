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
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/ui"
	log "github.com/Sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

const (
	pushInterval = 1 * time.Second

	defaultURL = "http://localhost:8086/k6"
)

var _ lib.AuthenticatedCollector = &Collector{}

type Config struct {
	DefaultURL null.String `json:"default_url,omitempty"`
}

type Collector struct {
	u          *url.URL
	client     client.Client
	batchConf  client.BatchPointsConfig
	buffer     []stats.Sample
	bufferLock sync.Mutex
}

func New(s string, conf_ interface{}, opts lib.Options) (*Collector, error) {
	conf := conf_.(*Config)

	if s == "" {
		s = conf.DefaultURL.String
	}
	if s == "" {
		s = defaultURL
	}

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

func (c *Collector) Init() error {
	// Try to create the database if it doesn't exist. Failure to do so is USUALLY harmless; it
	// usually means we're either a non-admin user to an existing DB or connecting over UDP.
	_, err := c.client.Query(client.NewQuery("CREATE DATABASE "+c.batchConf.Database, "", ""))
	if err != nil {
		log.WithError(err).Debug("InfluxDB: Couldn't create database; most likely harmless")
	}

	return nil
}

func (c *Collector) MakeConfig() interface{} {
	return &Config{}
}

func (c *Collector) Login(conf_ interface{}, in io.Reader, out io.Writer) (interface{}, error) {
	conf := conf_.(*Config)

	form := ui.Form{
		Fields: []ui.Field{
			ui.StringField{
				Key:     "host",
				Label:   "host",
				Default: "http://localhost:8086",
			},
			ui.StringField{
				Key:     "db",
				Label:   "database",
				Default: "k6",
			},
			ui.StringField{
				Key:   "username",
				Label: "username",
			},
			ui.StringField{
				Key:   "password",
				Label: "password",
			},
		},
	}
	data, err := form.Run(in, out)
	if err != nil {
		return nil, err
	}
	host := data["host"].(string)
	db := data["db"].(string)
	username := data["username"].(string)
	password := data["password"].(string)

	u, err := url.Parse(host + "/" + db)
	if err != nil {
		return nil, err
	}
	if username != "" {
		if password != "" {
			u.User = url.UserPassword(username, password)
		} else {
			u.User = url.User(username)
		}
	}

	cl, _, err := parseURL(u)
	if err != nil {
		return nil, err
	}
	if _, _, err := cl.Ping(5 * time.Second); err != nil {
		return nil, err
	}

	conf.DefaultURL = null.StringFrom(u.String())
	fmt.Fprint(out, color.New(color.Faint).Sprint("\n  to use this database: ")+color.CyanString("k6 run ")+color.New(color.FgHiCyan).Sprint("-o influxdb")+color.CyanString(" ...\n"))

	return conf, nil
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

func (c *Collector) IsReady() bool {
	return true
}

func (c *Collector) Collect(samples []stats.Sample) {
	c.bufferLock.Lock()
	c.buffer = append(c.buffer, samples...)
	c.bufferLock.Unlock()
}

func (c *Collector) commit() {
	c.bufferLock.Lock()
	samples := c.buffer
	c.buffer = nil
	c.bufferLock.Unlock()

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
