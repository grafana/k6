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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

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
				assert.True(t, expData.Time.Equal(receivedData.Time))
				assert.Equal(t, expData.Type, receivedData.Type)
				assert.Equal(t, expData.Value, receivedData.Value)
			case *SampleDataMap:
				receivedData, ok := receivedSample.Data.(*SampleDataMap)
				assert.True(t, ok)
				assert.True(t, expData.Tags.IsEqual(receivedData.Tags))
				assert.True(t, expData.Time.Equal(receivedData.Time))
				assert.Equal(t, expData.Type, receivedData.Type)
				assert.Equal(t, expData.Values, receivedData.Values)
			case *SampleDataAggregatedHTTPReqs:
				receivedData, ok := receivedSample.Data.(*SampleDataAggregatedHTTPReqs)
				assert.True(t, ok)
				assert.True(t, expData.Tags.IsEqual(receivedData.Tags))
				assert.True(t, expData.Time.Equal(receivedData.Time))
				assert.Equal(t, expData.Type, receivedData.Type)
				assert.Equal(t, expData.Values, receivedData.Values)
			default:
				t.Errorf("Unknown data type %#v", expData)
			}
		}
	}
}

func skewTrail(t netext.Trail, minCoef, maxCoef float64) netext.Trail {
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
	t.StartTime = t.EndTime.Add(-t.Duration)
	return t
}

func TestCloudCollector(t *testing.T) {
	t.Parallel()
	tb := testutils.NewHTTPMultiBin(t)
	tb.Mux.HandleFunc("/v1/tests", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{
			"reference_id": "123",
			"config": {
				"metricPushInterval": "10ms",
				"aggregationPeriod": "30ms",
				"aggregationCalcInterval": "40ms",
				"aggregationWaitPeriod": "5ms"
			}
		}`)
	}))
	defer tb.Cleanup()

	script := &lib.SourceData{
		Data:     []byte(""),
		Filename: "/script.js",
	}

	options := lib.Options{
		External: map[string]json.RawMessage{
			"loadimpact": json.RawMessage(tb.Replacer.Replace(`{
				"host": "HTTPBIN_IP_URL",
				"noCompress": true
			}`)),
		},
	}

	collector, err := New(NewConfig(), script, options, "1.0")
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
	tags := stats.IntoSampleTags(&map[string]string{"test": "mest", "a": "b"})

	expSamples := make(chan []Sample)
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", collector.referenceID), getSampleChecker(t, expSamples))

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
			Time:  Timestamp(now),
			Tags:  tags,
			Value: 1.0,
		},
	}}

	simpleTrail := netext.Trail{
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

	smallSkew := 0.05

	trails := []stats.SampleContainer{}
	for i := int64(0); i < collector.config.AggregationMinSamples.Int64; i++ {
		similarTrail := skewTrail(simpleTrail, 1.0, 1.0+smallSkew)
		trails = append(trails, &similarTrail)
	}

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
				assert.True(t, aggrData.Tags.IsEqual(tags))
				assert.Equal(t, collector.config.AggregationMinSamples.Int64, int64(aggrData.Count))
				assert.Equal(t, "aggregated_trend", aggrData.Type)
				assert.InDelta(t, now.UnixNano(), time.Time(aggrData.Time).UnixNano(), float64(collector.config.AggregationPeriod.Duration))

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
