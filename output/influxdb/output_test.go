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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

func TestBadConcurrentWrites(t *testing.T) {
	t.Parallel()
	logger := testutils.NewLogger(t)
	t.Run("0", func(t *testing.T) {
		t.Parallel()
		_, err := New(output.Params{
			Logger:         logger,
			ConfigArgument: "?concurrentWrites=0",
		})
		require.Error(t, err)
		require.Equal(t, err.Error(), "influxdb's ConcurrentWrites must be a positive number")
	})

	t.Run("-2", func(t *testing.T) {
		t.Parallel()
		_, err := New(output.Params{
			Logger:         logger,
			ConfigArgument: "?concurrentWrites=-2",
		})
		require.Error(t, err)
		require.Equal(t, err.Error(), "influxdb's ConcurrentWrites must be a positive number")
	})

	t.Run("2", func(t *testing.T) {
		t.Parallel()
		_, err := New(output.Params{
			Logger:         logger,
			ConfigArgument: "?concurrentWrites=2",
		})
		require.NoError(t, err)
	})
}

func testOutputCycle(t testing.TB, handler http.HandlerFunc, body func(testing.TB, *Output)) {
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

	c, err := newOutput(output.Params{
		Logger:         testutils.NewLogger(t),
		ConfigArgument: "http://" + l.Addr().String(),
	})
	require.NoError(t, err)

	require.NoError(t, c.Start())
	body(t, c)

	require.NoError(t, c.Stop())
}

func TestOutput(t *testing.T) {
	t.Parallel()
	var samplesRead int
	defer func() {
		require.Equal(t, samplesRead, 20)
	}()
	testOutputCycle(t, func(rw http.ResponseWriter, r *http.Request) {
		b := bytes.NewBuffer(nil)
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
	}, func(tb testing.TB, c *Output) {
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
		c.AddMetricSamples([]stats.SampleContainer{samples})
		c.AddMetricSamples([]stats.SampleContainer{samples})
	})
}

func TestExtractTagsToValues(t *testing.T) {
	t.Parallel()
	o, err := newOutput(output.Params{
		Logger:         testutils.NewLogger(t),
		ConfigArgument: "?tagsAsFields=stringField&tagsAsFields=stringField2:string&tagsAsFields=boolField:bool&tagsAsFields=floatField:float&tagsAsFields=intField:int",
	})
	require.NoError(t, err)
	tags := map[string]string{
		"stringField":  "string",
		"stringField2": "string2",
		"boolField":    "true",
		"floatField":   "3.14",
		"intField":     "12345",
	}
	values := o.extractTagsToValues(tags, map[string]interface{}{})

	require.Equal(t, "string", values["stringField"])
	require.Equal(t, "string2", values["stringField2"])
	require.Equal(t, true, values["boolField"])
	require.Equal(t, 3.14, values["floatField"])
	require.Equal(t, int64(12345), values["intField"])
}
