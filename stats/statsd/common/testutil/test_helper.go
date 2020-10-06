/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package testutil

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/statsd/common"
)

type getCollectorFn func(
	logger logrus.FieldLogger,
	addr, namespace null.String,
	bufferSize null.Int,
	pushInterval types.NullDuration,
) (*common.Collector, error)

// BaseTest is a helper function to test statsd/datadog collector
func BaseTest(t *testing.T,
	getCollector getCollectorFn,
	checkResult func(t *testing.T, samples []stats.SampleContainer, expectedOutput, output string),
) {
	t.Helper()
	testNamespace := "testing.things." // to be dynamic

	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	require.NoError(t, err)
	listener, err := net.ListenUDP("udp", addr) // we want to listen on a random port
	require.NoError(t, err)
	ch := make(chan string, 20)
	end := make(chan struct{})
	defer close(end)

	go func() {
		defer close(ch)
		var buf [4096]byte
		for {
			select {
			case <-end:
				return
			default:
				n, _, err := listener.ReadFromUDP(buf[:])
				require.NoError(t, err)
				ch <- string(buf[:n])
			}
		}
	}()

	pushInterval := types.NullDurationFrom(time.Millisecond * 10)
	collector, err := getCollector(
		testutils.NewLogger(t),
		null.StringFrom(listener.LocalAddr().String()),
		null.StringFrom(testNamespace),
		null.IntFrom(5),
		pushInterval,
	)
	require.NoError(t, err)
	require.NoError(t, collector.Init())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go collector.Run(ctx)
	newSample := func(m *stats.Metric, value float64, tags map[string]string) stats.Sample {
		return stats.Sample{
			Time:   time.Now(),
			Metric: m, Value: value, Tags: stats.IntoSampleTags(&tags),
		}
	}

	myCounter := stats.New("my_counter", stats.Counter)
	myGauge := stats.New("my_gauge", stats.Gauge)
	myTrend := stats.New("my_trend", stats.Trend)
	myRate := stats.New("my_rate", stats.Rate)
	myCheck := stats.New("my_check", stats.Rate)
	testMatrix := []struct {
		input  []stats.SampleContainer
		output string
	}{
		{
			input: []stats.SampleContainer{
				newSample(myCounter, 12, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_counter:12|c",
		},
		{
			input: []stats.SampleContainer{
				newSample(myGauge, 13, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_gauge:13.000000|g",
		},
		{
			input: []stats.SampleContainer{
				newSample(myTrend, 14, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_trend:14.000000|ms",
		},
		{
			input: []stats.SampleContainer{
				newSample(myRate, 15, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_rate:15|c",
		},
		{
			input: []stats.SampleContainer{
				newSample(myCheck, 16, map[string]string{
					"tag1":  "value1",
					"tag3":  "value3",
					"check": "max<100",
				}),
				newSample(myCheck, 0, map[string]string{
					"tag1":  "value1",
					"tag3":  "value3",
					"check": "max>100",
				}),
			},
			output: "testing.things.check.max<100.pass:1|c\ntesting.things.check.max>100.fail:1|c",
		},
	}
	for _, test := range testMatrix {
		collector.Collect(test.input)
		time.Sleep((time.Duration)(pushInterval.Duration))
		output := <-ch
		checkResult(t, test.input, test.output, output)
	}
}
