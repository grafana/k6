/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
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
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/loader"
	"github.com/loadimpact/k6/stats"
)

func tagEqual(expected, got *stats.SampleTags) bool {
	expectedMap := expected.CloneTags()
	gotMap := got.CloneTags()

	if len(expectedMap) != len(gotMap) {
		return false
	}

	for k, v := range gotMap {
		if k == "url" {
			if expectedMap["name"] != v {
				return false
			}
		} else if expectedMap[k] != v {
			return false
		}
	}
	return true
}

func getSampleChecker(t *testing.T, expSamples <-chan []Sample) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		receivedSamples := []Sample{}
		assert.NoError(t, json.Unmarshal(body, &receivedSamples))

		expSamples := <-expSamples
		if !assert.Len(t, receivedSamples, len(expSamples)) {
			return
		}

		for i, expSample := range expSamples {
			receivedSample := receivedSamples[i]
			assert.Equal(t, expSample.Metric, receivedSample.Metric)
			assert.Equal(t, expSample.Type, receivedSample.Type)

			if callbackCheck, ok := expSample.Data.(func(interface{})); ok {
				callbackCheck(receivedSample.Data)
				continue
			}

			if !assert.IsType(t, expSample.Data, receivedSample.Data) {
				continue
			}

			switch expData := expSample.Data.(type) {
			case *SampleDataSingle:
				receivedData, ok := receivedSample.Data.(*SampleDataSingle)
				assert.True(t, ok)
				assert.True(t, expData.Tags.IsEqual(receivedData.Tags))
				assert.Equal(t, expData.Time, receivedData.Time)
				assert.Equal(t, expData.Type, receivedData.Type)
				assert.Equal(t, expData.Value, receivedData.Value)
			case *SampleDataMap:
				receivedData, ok := receivedSample.Data.(*SampleDataMap)
				assert.True(t, ok)
				assert.True(t, tagEqual(expData.Tags, receivedData.Tags))
				assert.Equal(t, expData.Time, receivedData.Time)
				assert.Equal(t, expData.Type, receivedData.Type)
				assert.Equal(t, expData.Values, receivedData.Values)
			case *SampleDataAggregatedHTTPReqs:
				receivedData, ok := receivedSample.Data.(*SampleDataAggregatedHTTPReqs)
				assert.True(t, ok)
				assert.True(t, expData.Tags.IsEqual(receivedData.Tags))
				assert.Equal(t, expData.Time, receivedData.Time)
				assert.Equal(t, expData.Type, receivedData.Type)
				assert.Equal(t, expData.Values, receivedData.Values)
			default:
				t.Errorf("Unknown data type %#v", expData)
			}
		}
	}
}

func skewTrail(t httpext.Trail, minCoef, maxCoef float64) httpext.Trail {
	coef := minCoef + rand.Float64()*(maxCoef-minCoef)
	addJitter := func(d *time.Duration) {
		*d = time.Duration(float64(*d) * coef)
	}
	addJitter(&t.Blocked)
	addJitter(&t.Connecting)
	addJitter(&t.TLSHandshaking)
	addJitter(&t.Sending)
	addJitter(&t.Waiting)
	addJitter(&t.Receiving)
	t.ConnDuration = t.Connecting + t.TLSHandshaking
	t.Duration = t.Sending + t.Waiting + t.Receiving
	return t
}

func TestCloudCollector(t *testing.T) {
	t.Parallel()

	getTestRunner := func(minSamples int) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			runCloudCollectorTestCase(t, minSamples)
		}
	}

	for tcNum, minSamples := range []int{60, 75, 100} {
		t.Run(fmt.Sprintf("tc%d_minSamples%d", tcNum, minSamples), getTestRunner(minSamples))
	}
}

func runCloudCollectorTestCase(t *testing.T, minSamples int) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	tb.Mux.HandleFunc("/v1/tests", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
			"reference_id": "123",
			"config": {
				"metricPushInterval": "10ms",
				"aggregationPeriod": "30ms",
				"aggregationCalcInterval": "40ms",
				"aggregationWaitPeriod": "5ms",
				"aggregationMinSamples": %d
			}
		}`, minSamples)
		require.NoError(t, err)
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
	collector, err := New(testutils.NewLogger(t), config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(t, err)

	assert.True(t, collector.config.Host.Valid)
	assert.Equal(t, tb.ServerHTTP.URL, collector.config.Host.String)
	assert.True(t, collector.config.NoCompress.Valid)
	assert.True(t, collector.config.NoCompress.Bool)
	assert.False(t, collector.config.MetricPushInterval.Valid)
	assert.False(t, collector.config.AggregationPeriod.Valid)
	assert.False(t, collector.config.AggregationWaitPeriod.Valid)

	require.NoError(t, collector.Init())
	assert.Equal(t, "123", collector.referenceID)
	assert.True(t, collector.config.MetricPushInterval.Valid)
	assert.Equal(t, types.Duration(10*time.Millisecond), collector.config.MetricPushInterval.Duration)
	assert.True(t, collector.config.AggregationPeriod.Valid)
	assert.Equal(t, types.Duration(30*time.Millisecond), collector.config.AggregationPeriod.Duration)
	assert.True(t, collector.config.AggregationWaitPeriod.Valid)
	assert.Equal(t, types.Duration(5*time.Millisecond), collector.config.AggregationWaitPeriod.Duration)

	now := time.Now()
	tagMap := map[string]string{"test": "mest", "a": "b", "name": "name", "url": "url"}
	tags := stats.IntoSampleTags(&tagMap)
	expectedTagMap := tags.CloneTags()
	expectedTagMap["url"], _ = tags.Get("name")
	expectedTags := stats.IntoSampleTags(&expectedTagMap)

	expSamples := make(chan []Sample)
	defer close(expSamples)
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", collector.referenceID), getSampleChecker(t, expSamples))
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/tests/%s", collector.referenceID), func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK) // silence a test warning
	})

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		collector.Run(ctx)
		wg.Done()
	}()

	collector.Collect([]stats.SampleContainer{stats.Sample{
		Time:   now,
		Metric: metrics.VUs,
		Tags:   tags,
		Value:  1.0,
	}})
	expSamples <- []Sample{{
		Type:   DataTypeSingle,
		Metric: metrics.VUs.Name,
		Data: &SampleDataSingle{
			Type:  metrics.VUs.Type,
			Time:  toMicroSecond(now),
			Tags:  tags,
			Value: 1.0,
		},
	}}

	simpleTrail := httpext.Trail{
		Blocked:        100 * time.Millisecond,
		Connecting:     200 * time.Millisecond,
		TLSHandshaking: 300 * time.Millisecond,
		Sending:        400 * time.Millisecond,
		Waiting:        500 * time.Millisecond,
		Receiving:      600 * time.Millisecond,

		EndTime:      now,
		ConnDuration: 500 * time.Millisecond,
		Duration:     1500 * time.Millisecond,
		Tags:         tags,
	}
	collector.Collect([]stats.SampleContainer{&simpleTrail})
	expSamples <- []Sample{*NewSampleFromTrail(&simpleTrail)}

	smallSkew := 0.02

	trails := []stats.SampleContainer{}
	durations := make([]time.Duration, len(trails))
	for i := int64(0); i < collector.config.AggregationMinSamples.Int64; i++ {
		similarTrail := skewTrail(simpleTrail, 1.0, 1.0+smallSkew)
		trails = append(trails, &similarTrail)
		durations = append(durations, similarTrail.Duration)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("Sorted durations: %#v", durations) // Useful to debug any failures, doesn't get in the way otherwise

	checkAggrMetric := func(normal time.Duration, aggr AggregatedMetric) {
		assert.True(t, aggr.Min <= aggr.Avg)
		assert.True(t, aggr.Avg <= aggr.Max)
		assert.InEpsilon(t, normal, stats.ToD(aggr.Min), smallSkew)
		assert.InEpsilon(t, normal, stats.ToD(aggr.Avg), smallSkew)
		assert.InEpsilon(t, normal, stats.ToD(aggr.Max), smallSkew)
	}

	outlierTrail := skewTrail(simpleTrail, 2.0+smallSkew, 3.0+smallSkew)
	trails = append(trails, &outlierTrail)
	collector.Collect(trails)
	expSamples <- []Sample{
		*NewSampleFromTrail(&outlierTrail),
		{
			Type:   DataTypeAggregatedHTTPReqs,
			Metric: "http_req_li_all",
			Data: func(data interface{}) {
				aggrData, ok := data.(*SampleDataAggregatedHTTPReqs)
				assert.True(t, ok)
				assert.True(t, aggrData.Tags.IsEqual(expectedTags))
				assert.Equal(t, collector.config.AggregationMinSamples.Int64, int64(aggrData.Count))
				assert.Equal(t, "aggregated_trend", aggrData.Type)
				assert.InDelta(t, now.UnixNano(), aggrData.Time*1000, float64(collector.config.AggregationPeriod.Duration))

				checkAggrMetric(simpleTrail.Duration, aggrData.Values.Duration)
				checkAggrMetric(simpleTrail.Blocked, aggrData.Values.Blocked)
				checkAggrMetric(simpleTrail.Connecting, aggrData.Values.Connecting)
				checkAggrMetric(simpleTrail.TLSHandshaking, aggrData.Values.TLSHandshaking)
				checkAggrMetric(simpleTrail.Sending, aggrData.Values.Sending)
				checkAggrMetric(simpleTrail.Waiting, aggrData.Values.Waiting)
				checkAggrMetric(simpleTrail.Receiving, aggrData.Values.Receiving)
			},
		},
	}

	cancel()
	wg.Wait()
}

func TestCloudCollectorMaxPerPacket(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	maxMetricSamplesPerPackage := 20
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
		require.NoError(t, err)
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
	collector, err := New(testutils.NewLogger(t), config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(t, err)
	now := time.Now()
	tags := stats.IntoSampleTags(&map[string]string{"test": "mest", "a": "b"})
	gotTheLimit := false
	var m sync.Mutex

	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", collector.referenceID),
		func(_ http.ResponseWriter, r *http.Request) {
			body, err := ioutil.ReadAll(r.Body)
			assert.NoError(t, err)
			receivedSamples := []Sample{}
			assert.NoError(t, json.Unmarshal(body, &receivedSamples))
			assert.True(t, len(receivedSamples) <= maxMetricSamplesPerPackage)
			if len(receivedSamples) == maxMetricSamplesPerPackage {
				m.Lock()
				gotTheLimit = true
				m.Unlock()
			}
		})

	require.NoError(t, collector.Init())
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		collector.Run(ctx)
		wg.Done()
	}()

	collector.Collect([]stats.SampleContainer{stats.Sample{
		Time:   now,
		Metric: metrics.VUs,
		Tags:   stats.NewSampleTags(tags.CloneTags()),
		Value:  1.0,
	}})
	for j := time.Duration(1); j <= 200; j++ {
		container := make([]stats.SampleContainer, 0, 500)
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

	cancel()
	wg.Wait()
	require.True(t, gotTheLimit)
}

func TestCloudCollectorStopSendingMetric(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	tb.Mux.HandleFunc("/v1/tests", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprint(w, `{
			"reference_id": "12",
			"config": {
				"metricPushInterval": "200ms",
				"aggregationPeriod": "100ms",
				"maxMetricSamplesPerPackage": 20,
				"aggregationCalcInterval": "100ms",
				"aggregationWaitPeriod": "100ms"
			}
		}`)
		require.NoError(t, err)
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
		Host:                       null.StringFrom(tb.ServerHTTP.URL),
		NoCompress:                 null.BoolFrom(true),
		MaxMetricSamplesPerPackage: null.IntFrom(50),
	})
	collector, err := New(testutils.NewLogger(t), config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(t, err)
	now := time.Now()
	tags := stats.IntoSampleTags(&map[string]string{"test": "mest", "a": "b"})

	count := 1
	max := 5
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", collector.referenceID),
		func(w http.ResponseWriter, r *http.Request) {
			count++
			if count == max {
				type payload struct {
					Error ErrorResponse `json:"error"`
				}
				res := &payload{}
				res.Error = ErrorResponse{Code: 4}
				w.Header().Set("Content-Type", "application/json")
				data, err := json.Marshal(res)
				if err != nil {
					t.Fatal(err)
				}
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write(data)
				return
			}
			body, err := ioutil.ReadAll(r.Body)
			assert.NoError(t, err)
			receivedSamples := []Sample{}
			assert.NoError(t, json.Unmarshal(body, &receivedSamples))
		})

	require.NoError(t, collector.Init())
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		collector.Run(ctx)
		wg.Done()
	}()

	collector.Collect([]stats.SampleContainer{stats.Sample{
		Time:   now,
		Metric: metrics.VUs,
		Tags:   stats.NewSampleTags(tags.CloneTags()),
		Value:  1.0,
	}})
	for j := time.Duration(1); j <= 200; j++ {
		container := make([]stats.SampleContainer, 0, 500)
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

	cancel()
	wg.Wait()
	require.Equal(t, lib.RunStatusQueued, collector.runStatus)
	select {
	case <-collector.stopSendingMetricsCh:
		// all is fine
	default:
		t.Fatal("sending metrics wasn't stopped")
	}
	require.Equal(t, max, count)

	nBufferSamples := len(collector.bufferSamples)
	nBufferHTTPTrails := len(collector.bufferHTTPTrails)
	collector.Collect([]stats.SampleContainer{stats.Sample{
		Time:   now,
		Metric: metrics.VUs,
		Tags:   stats.NewSampleTags(tags.CloneTags()),
		Value:  1.0,
	}})
	if nBufferSamples != len(collector.bufferSamples) || nBufferHTTPTrails != len(collector.bufferHTTPTrails) {
		t.Errorf("Collector still collects data after stop sending metrics")
	}
}

func TestCloudCollectorAggregationPeriodZeroNoBlock(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	tb.Mux.HandleFunc("/v1/tests", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
			"reference_id": "123",
			"config": {
				"metricPushInterval": "10ms",
				"aggregationPeriod": "0ms",
				"aggregationCalcInterval": "40ms",
				"aggregationWaitPeriod": "5ms"
			}
		}`)
		require.NoError(t, err)
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
	collector, err := New(testutils.NewLogger(t), config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(t, err)

	assert.True(t, collector.config.Host.Valid)
	assert.Equal(t, tb.ServerHTTP.URL, collector.config.Host.String)
	assert.True(t, collector.config.NoCompress.Valid)
	assert.True(t, collector.config.NoCompress.Bool)
	assert.False(t, collector.config.MetricPushInterval.Valid)
	assert.False(t, collector.config.AggregationPeriod.Valid)
	assert.False(t, collector.config.AggregationWaitPeriod.Valid)

	require.NoError(t, collector.Init())
	assert.Equal(t, "123", collector.referenceID)
	assert.True(t, collector.config.MetricPushInterval.Valid)
	assert.Equal(t, types.Duration(10*time.Millisecond), collector.config.MetricPushInterval.Duration)
	assert.True(t, collector.config.AggregationPeriod.Valid)
	assert.Equal(t, types.Duration(0), collector.config.AggregationPeriod.Duration)
	assert.True(t, collector.config.AggregationWaitPeriod.Valid)
	assert.Equal(t, types.Duration(5*time.Millisecond), collector.config.AggregationWaitPeriod.Duration)

	expSamples := make(chan []Sample)
	defer close(expSamples)
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", collector.referenceID), getSampleChecker(t, expSamples))

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		collector.Run(ctx)
		wg.Done()
	}()

	cancel()
	wg.Wait()
	require.Equal(t, lib.RunStatusQueued, collector.runStatus)
}

func TestCloudCollectorRecvIterLIAllIterations(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	tb.Mux.HandleFunc("/v1/tests", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{"reference_id": "123"}`)
		require.NoError(t, err)
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
	collector, err := New(testutils.NewLogger(t), config, script, options, []lib.ExecutionStep{}, "1.0")
	require.NoError(t, err)

	gotIterations := false
	var m sync.Mutex
	expValues := map[string]float64{
		"data_received":      100,
		"data_sent":          200,
		"iteration_duration": 60000,
		"iterations":         1,
	}

	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", collector.referenceID),
		func(_ http.ResponseWriter, r *http.Request) {
			body, err := ioutil.ReadAll(r.Body)
			assert.NoError(t, err)

			receivedSamples := []Sample{}
			assert.NoError(t, json.Unmarshal(body, &receivedSamples))

			assert.Len(t, receivedSamples, 1)
			assert.Equal(t, "iter_li_all", receivedSamples[0].Metric)
			assert.Equal(t, DataTypeMap, receivedSamples[0].Type)
			data, ok := receivedSamples[0].Data.(*SampleDataMap)
			assert.True(t, ok)
			assert.Equal(t, expValues, data.Values)

			m.Lock()
			gotIterations = true
			m.Unlock()
		})

	require.NoError(t, collector.Init())
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		collector.Run(ctx)
		wg.Done()
	}()

	now := time.Now()
	simpleNetTrail := netext.NetTrail{
		BytesRead:     100,
		BytesWritten:  200,
		FullIteration: true,
		StartTime:     now.Add(-time.Minute),
		EndTime:       now,
		Samples: []stats.Sample{
			{
				Time:   now,
				Metric: metrics.DataSent,
				Value:  float64(200),
			},
			{
				Time:   now,
				Metric: metrics.DataReceived,
				Value:  float64(100),
			},
			{
				Time:   now,
				Metric: metrics.Iterations,
				Value:  1,
			},
		},
	}

	collector.Collect([]stats.SampleContainer{&simpleNetTrail})

	cancel()
	wg.Wait()
	require.True(t, gotIterations)
}

func TestNewName(t *testing.T) {
	t.Parallel()
	mustParse := func(u string) *url.URL {
		result, err := url.Parse(u)
		require.NoError(t, err)
		return result
	}

	cases := []struct {
		url      *url.URL
		expected string
	}{
		{
			url: &url.URL{
				Opaque: "github.com/loadimpact/k6/samples/http_get.js",
			},
			expected: "http_get.js",
		},
		{
			url:      mustParse("http://github.com/loadimpact/k6/samples/http_get.js"),
			expected: "http_get.js",
		},
		{
			url:      mustParse("file://home/user/k6/samples/http_get.js"),
			expected: "http_get.js",
		},
		{
			url:      mustParse("file://C:/home/user/k6/samples/http_get.js"),
			expected: "http_get.js",
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		t.Run(testCase.url.String(), func(t *testing.T) {
			script := &loader.SourceData{
				URL: testCase.url,
			}
			collector, err := New(testutils.NewLogger(t), NewConfig(), script, lib.Options{
				Duration: types.NullDurationFrom(1 * time.Second),
			}, []lib.ExecutionStep{}, "1.0")
			require.NoError(t, err)
			require.Equal(t, collector.config.Name.String, testCase.expected)
		})
	}
}
