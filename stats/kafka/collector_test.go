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
	"testing"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/stats"
)

func TestRun(t *testing.T) {
	broker := sarama.NewMockBroker(t, 1)
	coordinator := sarama.NewMockBroker(t, 2)
	seedMeta := new(sarama.MetadataResponse)
	seedMeta.AddBroker(coordinator.Addr(), coordinator.BrokerID())
	seedMeta.AddTopicPartition("my_topic", 0, 1, []int32{}, []int32{}, sarama.ErrNoError)
	broker.Returns(seedMeta)

	cfg := Config{
		Brokers: []string{broker.Addr()},
		Topic:   null.NewString("my_topic", false),
	}
	config := NewConfig().Apply(cfg)
	c, err := New(testutils.NewLogger(t), config)

	assert.Nil(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Run(ctx)
	}()
	cancel()
	wg.Wait()
}

func TestFormatSamples(t *testing.T) {
	c := Collector{}
	c.Config.InfluxDBConfig.ConcurrentWrites = null.IntFrom(10)
	metric := stats.New("my_metric", stats.Gauge)
	samples := stats.Samples{
		{Metric: metric, Value: 1.25, Tags: stats.IntoSampleTags(&map[string]string{"a": "1"})},
		{Metric: metric, Value: 2, Tags: stats.IntoSampleTags(&map[string]string{"b": "2"})},
	}

	c.Config.Format = null.NewString("influxdb", false)
	fmtdSamples, err := c.formatSamples(samples)

	assert.Nil(t, err)
	assert.Equal(t, []string{"my_metric,a=1 value=1.25", "my_metric,b=2 value=2"}, fmtdSamples)

	c.Config.Format = null.NewString("json", false)
	fmtdSamples, err = c.formatSamples(samples)

	expJSON1 := "{\"type\":\"Point\",\"data\":{\"time\":\"0001-01-01T00:00:00Z\",\"value\":1.25,\"tags\":{\"a\":\"1\"}},\"metric\":\"my_metric\"}"
	expJSON2 := "{\"type\":\"Point\",\"data\":{\"time\":\"0001-01-01T00:00:00Z\",\"value\":2,\"tags\":{\"b\":\"2\"}},\"metric\":\"my_metric\"}"

	assert.Nil(t, err)
	assert.Equal(t, []string{expJSON1, expJSON2}, fmtdSamples)
}
