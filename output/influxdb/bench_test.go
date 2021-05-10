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

package influxdb

import (
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"go.k6.io/k6/stats"
)

func benchmarkInfluxdb(b *testing.B, t time.Duration) {
	testOutputCycle(b, func(rw http.ResponseWriter, r *http.Request) {
		for {
			time.Sleep(t)
			m, _ := io.CopyN(ioutil.Discard, r.Body, 1<<18) // read 1/4 mb a time
			if m == 0 {
				break
			}
		}
		rw.WriteHeader(204)
	}, func(tb testing.TB, c *Output) {
		b = tb.(*testing.B)
		b.ResetTimer()

		samples := make(stats.Samples, 10)
		for i := 0; i < len(samples); i++ {
			samples[i] = stats.Sample{
				Metric: stats.New("testGauge", stats.Gauge),
				Time:   time.Now(),
				Tags: stats.NewSampleTags(map[string]string{
					"something": "else",
					"VU":        "21",
					"else":      "something",
				}),
				Value: 2.0,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.AddMetricSamples([]stats.SampleContainer{samples})
			time.Sleep(time.Nanosecond * 20)
		}
	})
}

func BenchmarkInfluxdb1Second(b *testing.B) {
	benchmarkInfluxdb(b, time.Second)
}

func BenchmarkInfluxdb2Second(b *testing.B) {
	benchmarkInfluxdb(b, 2*time.Second)
}

func BenchmarkInfluxdb100Milliseconds(b *testing.B) {
	benchmarkInfluxdb(b, 100*time.Millisecond)
}
