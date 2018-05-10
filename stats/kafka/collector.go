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

package kafka

import (
	"context"
	"sync"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/influxdb"
	log "github.com/sirupsen/logrus"
)

const (
	pushInterval = 1 * time.Second
)

// Collector implements the lib.Collector interface and should be used only for testing
type Collector struct {
	Producer *kafka.Producer
	Config   Config

	Samples  []stats.Sample
	lock     sync.Mutex
}

// New creates an instance of the collector
func New(conf Config) (*Collector, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": strings.Join(conf.Brokers, ",")})
	if err != nil {
		return nil, err
	}

	return &Collector{
		Producer:  p,
		Config:    conf,
	}, nil
}

// Init does nothing, it's only included to satisfy the lib.Collector interface
func (c *Collector) Init() error { return nil }

// Run just blocks until the context is done
func (c *Collector) Run(ctx context.Context) {
	log.Debug("Kafka: Running!")
	ticker := time.NewTicker(pushInterval)
	for {
		select {
		case <-ticker.C:
			c.pushMetrics()
		case <-ctx.Done():
			c.pushMetrics()
			return
		}
	}
}

// Collect just appends all of the samples passed to it to the internal sample slice.
// According to the the lib.Collector interface, it should never be called concurrently,
// so there's no locking on purpose - that way Go's race condition detector can actually
// detect incorrect usage.
// Also, theoretically the collector doesn't have to actually Run() before samples start
// being collected, it only has to be initialized.
func (c *Collector) Collect(samples []stats.Sample) {
	c.lock.Lock()
	c.Samples = append(c.Samples, samples...)
	c.lock.Unlock()
}

// Link returns a dummy string, it's only included to satisfy the lib.Collector interface
func (c *Collector) Link() string {
	return ""
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() lib.TagSet {
	return lib.TagSet{} // There are no required tags for this collector
}

func (c *Collector) pushMetrics() {
	startTime := time.Now()

	c.lock.Lock()
	samples := c.Samples
	c.Samples = nil
	c.lock.Unlock()

	// Format the metrics
	var metrics []string
	if c.Config.Format == "influx" {
		i, err := influxdb.New(influxdb.Config{})
		if err != nil {
			log.WithError(err).Error("Kafka: Couldn't create influx collector")
			return
		}
		metrics, err = i.Format(samples)
		if err != nil {
			log.WithError(err).Error("Kafka: Couldn't format samples into influx")
			return
		}
	}

	// Send the metrics
	log.Debug("Kafka: Delivering...")

	// Delivery report handler for produced messages
	go func() {
		for e := range c.Producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					log.Debugf("Kafka: Delivery failed: %v\n", ev.TopicPartition)
				} else {
					log.Debugf("Kafka: Delivered message to %v\n", ev.TopicPartition)
				}
			}
		}
	}()

	// Produce messages to topic (asynchronously)
	for _, metric := range metrics {
		c.Producer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &c.Config.Topic, Partition: kafka.PartitionAny},
			Value:          []byte(metric),
		}, nil)
	}

	// Wait for message deliveries
	leftoverMessages := c.Producer.Flush(15 * 1000)
	if leftoverMessages > 0 {
		log.WithField("leftover messages", leftoverMessages).Warn("Kafka: Flush timed out.")
	}

	t := time.Since(startTime)
	log.WithField("t", t).Debug("Kafka: Delivered!")
}
