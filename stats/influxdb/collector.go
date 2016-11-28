package influxdb

import (
	"context"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/loadimpact/speedboat/stats"
	"net/url"
	"time"
)

const pushInterval = 1 * time.Second

type Collector struct {
	u         *url.URL
	client    client.Client
	batchConf client.BatchPointsConfig
	buffer    []stats.Sample
}

func New(u *url.URL) (*Collector, error) {
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
