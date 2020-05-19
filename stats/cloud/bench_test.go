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

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

// script to clean the logs: `perl -p -e  "s/time=\".*\n//g"`
// TODO: find what sed magic needs to be used to make it work and use it in order to be able to do
// inplace
// TODO: Add a more versatile test not only with metrics that will be aggregated all the time and
// not only httpext.Trail
func BenchmarkCloud(b *testing.B) {
	tb := httpmultibin.NewHTTPMultiBin(b)
	var maxMetricSamplesPerPackage = 20
	tb.Mux.HandleFunc("/v1/tests", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
			"reference_id": "12",
			"config": {
				"metricPushInterval": "200ms",
				"aggregationPeriod": "100ms",
				"maxMetricSamplesPerPackage": %d,
				"aggregationCalcInterval": "100ms",
				"aggregationWaitPeriod": "100ms"
			}
		}`, maxMetricSamplesPerPackage)
		require.NoError(b, err)
	}))
	defer tb.Cleanup()

	script := &loader.SourceData{
		Data: []byte(""),
		URL:  &url.URL{Path: "/script.js"},
	}

	options := lib.Options{
		Duration: types.NullDurationFrom(1 * time.Second),
	}

	config := NewConfig().Apply(Config{
		Host:       null.StringFrom(tb.ServerHTTP.URL),
		NoCompress: null.BoolFrom(true),
	})
	collector, err := New(config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(b, err)
	now := time.Now()
	tags := stats.IntoSampleTags(&map[string]string{"test": "mest", "a": "b", "url": "something", "name": "else"})
	var gotTheLimit = false
	var m sync.Mutex

	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", collector.referenceID),
		func(_ http.ResponseWriter, r *http.Request) {
			body, err := ioutil.ReadAll(r.Body)
			assert.NoError(b, err)
			receivedSamples := []Sample{}
			assert.NoError(b, json.Unmarshal(body, &receivedSamples))
			assert.True(b, len(receivedSamples) <= maxMetricSamplesPerPackage)
			if len(receivedSamples) == maxMetricSamplesPerPackage {
				m.Lock()
				gotTheLimit = true
				m.Unlock()
			}
		})

	require.NoError(b, collector.Init())
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		collector.Run(ctx)
		wg.Done()
	}()

	for s := 0; s < b.N; s++ {
		for j := time.Duration(1); j <= 200; j++ {
			var container = make([]stats.SampleContainer, 0, 500)
			for i := time.Duration(1); i <= 50; i++ {
				container = append(container, &httpext.Trail{
					Blocked:        i % 200 * 100 * time.Millisecond,
					Connecting:     i % 200 * 200 * time.Millisecond,
					TLSHandshaking: i % 200 * 300 * time.Millisecond,
					Sending:        i * i * 400 * time.Millisecond,
					Waiting:        500 * time.Millisecond,
					Receiving:      600 * time.Millisecond,

					EndTime:      now.Add(i * 100),
					ConnDuration: 500 * time.Millisecond,
					Duration:     j * i * 1500 * time.Millisecond,
					Tags:         stats.NewSampleTags(tags.CloneTags()),
				})
			}
			collector.Collect(container)
		}
	}

	cancel()
	wg.Wait()
	require.True(b, gotTheLimit)
}
