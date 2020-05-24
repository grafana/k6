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
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v4"
)

func TestBadConcurrentWrites(t *testing.T) {
	c := NewConfig()
	t.Run("0", func(t *testing.T) {
		c.ConcurrentWrites = null.IntFrom(0)
		_, err := New(*c)
		require.Error(t, err)
		require.Equal(t, err.Error(), "influxdb's ConcurrentWrites must be a positive number")
	})

	t.Run("-2", func(t *testing.T) {
		c.ConcurrentWrites = null.IntFrom(-2)
		_, err := New(*c)
		require.Error(t, err)
		require.Equal(t, err.Error(), "influxdb's ConcurrentWrites must be a positive number")
	})

	t.Run("2", func(t *testing.T) {
		c.ConcurrentWrites = null.IntFrom(2)
		_, err := New(*c)
		require.NoError(t, err)
	})
}

func testCollectorCycle(t testing.TB, handler http.HandlerFunc, body func(testing.TB, *Collector)) {
	s := &http.Server{
		Addr:           ":",
		Handler:        handler,
		MaxHeaderBytes: 1 << 20,
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		_ = l.Close()
	}()

	defer func() {
		require.NoError(t, s.Shutdown(context.Background()))
	}()

	go func() {
		require.Equal(t, http.ErrServerClosed, s.Serve(l))
	}()

	config := NewConfig()
	config.Addr = null.StringFrom("http://" + l.Addr().String())
	c, err := New(*config)
	require.NoError(t, err)

	require.NoError(t, c.Init())
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer cancel()
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Run(ctx)
	}()

	body(t, c)

	cancel()
	wg.Wait()
}
func TestCollector(t *testing.T) {
	var samplesRead int
	defer func() {
		require.Equal(t, samplesRead, 20)
	}()
	testCollectorCycle(t, func(rw http.ResponseWriter, r *http.Request) {
		var b = bytes.NewBuffer(nil)
		_, _ = io.Copy(b, r.Body)
		for {
			s, err := b.ReadString('\n')
			if len(s) > 0 {
				samplesRead++
			}
			if err != nil {
				break
			}
		}

		rw.WriteHeader(204)
	}, func(tb testing.TB, c *Collector) {
		var samples = make(stats.Samples, 10)
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
		c.Collect([]stats.SampleContainer{samples})
		c.Collect([]stats.SampleContainer{samples})
	})

}
