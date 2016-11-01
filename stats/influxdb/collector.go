package influxdb

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/loadimpact/speedboat/stats"
	"net/url"
	"sync"
	"time"
)

const pushInterval = 1 * time.Second

type Collector struct {
	client    client.Client
	batchConf client.BatchPointsConfig

	buffers      []*Buffer
	buffersMutex sync.Mutex
}

func New(u *url.URL) (*Collector, error) {
	cl, batchConf, err := parseURL(u)
	if err != nil {
		return nil, err
	}

	return &Collector{
		client:    cl,
		batchConf: batchConf,
	}, nil
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

func (c *Collector) Buffer() stats.Buffer {
	buf := &(Buffer{})
	c.buffersMutex.Lock()
	c.buffers = append(c.buffers, buf)
	c.buffersMutex.Unlock()
	return buf
}

func (c *Collector) commit() {
	log.Debug("InfluxDB: Committing...")
	batch, err := client.NewBatchPoints(c.batchConf)
	if err != nil {
		log.WithError(err).Error("InfluxDB: Couldn't make a batch")
		return
	}

	buffers := c.buffers
	samples := []stats.Sample{}
	for _, buf := range buffers {
		samples = append(samples, buf.Drain()...)
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

	log.WithField("points", len(batch.Points())).Debug("InfluxDB: Writing points...")
	if err := c.client.Write(batch); err != nil {
		log.WithError(err).Error("InfluxDB: Couldn't write stats")
	}
}

type Buffer []stats.Sample

func (b *Buffer) Add(samples ...stats.Sample) {
	*b = append(*b, samples...)
}

func (b *Buffer) Drain() []stats.Sample {
	old := *b
	*b = (*b)[:0]
	return old
}
