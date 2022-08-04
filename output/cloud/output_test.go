package cloud

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

func tagEqual(expected, got *metrics.SampleTags) bool {
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

func skewTrail(r *rand.Rand, t httpext.Trail, minCoef, maxCoef float64) httpext.Trail {
	coef := minCoef + r.Float64()*(maxCoef-minCoef)
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

func TestCloudOutput(t *testing.T) {
	t.Parallel()

	getTestRunner := func(minSamples int) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			runCloudOutputTestCase(t, minSamples)
		}
	}

	for tcNum, minSamples := range []int{60, 75, 100} {
		t.Run(fmt.Sprintf("tc%d_minSamples%d", tcNum, minSamples), getTestRunner(minSamples))
	}
}

func runCloudOutputTestCase(t *testing.T, minSamples int) {
	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed)) //nolint:gosec
	t.Logf("Random source seeded with %d\n", seed)

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

	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())
	out, err := newOutput(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{"host": "%s", "noCompress": true}`, tb.ServerHTTP.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
	})
	require.NoError(t, err)

	assert.True(t, out.config.Host.Valid)
	assert.Equal(t, tb.ServerHTTP.URL, out.config.Host.String)
	assert.True(t, out.config.NoCompress.Valid)
	assert.True(t, out.config.NoCompress.Bool)
	assert.False(t, out.config.MetricPushInterval.Valid)
	assert.False(t, out.config.AggregationPeriod.Valid)
	assert.False(t, out.config.AggregationWaitPeriod.Valid)

	require.NoError(t, out.Start())
	assert.Equal(t, "123", out.referenceID)
	assert.True(t, out.config.MetricPushInterval.Valid)
	assert.Equal(t, types.Duration(10*time.Millisecond), out.config.MetricPushInterval.Duration)
	assert.True(t, out.config.AggregationPeriod.Valid)
	assert.Equal(t, types.Duration(30*time.Millisecond), out.config.AggregationPeriod.Duration)
	assert.True(t, out.config.AggregationWaitPeriod.Valid)
	assert.Equal(t, types.Duration(5*time.Millisecond), out.config.AggregationWaitPeriod.Duration)

	now := time.Now()
	tagMap := map[string]string{"test": "mest", "a": "b", "name": "name", "url": "url"}
	tags := metrics.IntoSampleTags(&tagMap)
	expectedTagMap := tags.CloneTags()
	expectedTagMap["url"], _ = tags.Get("name")
	expectedTags := metrics.IntoSampleTags(&expectedTagMap)

	expSamples := make(chan []Sample)
	defer close(expSamples)
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", out.referenceID), getSampleChecker(t, expSamples))
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/tests/%s", out.referenceID), func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK) // silence a test warning
	})

	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		Time:   now,
		Metric: builtinMetrics.VUs,
		Tags:   tags,
		Value:  1.0,
	}})
	expSamples <- []Sample{{
		Type:   DataTypeSingle,
		Metric: metrics.VUsName,
		Data: &SampleDataSingle{
			Type:  builtinMetrics.VUs.Type,
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
	out.AddMetricSamples([]metrics.SampleContainer{&simpleTrail})
	expSamples <- []Sample{*NewSampleFromTrail(&simpleTrail)}

	smallSkew := 0.02

	trails := []metrics.SampleContainer{}
	durations := make([]time.Duration, len(trails))
	for i := int64(0); i < out.config.AggregationMinSamples.Int64; i++ {
		similarTrail := skewTrail(r, simpleTrail, 1.0, 1.0+smallSkew)
		trails = append(trails, &similarTrail)
		durations = append(durations, similarTrail.Duration)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("Sorted durations: %#v", durations) // Useful to debug any failures, doesn't get in the way otherwise

	checkAggrMetric := func(normal time.Duration, aggr AggregatedMetric) {
		assert.True(t, aggr.Min <= aggr.Avg)
		assert.True(t, aggr.Avg <= aggr.Max)
		assert.InEpsilon(t, normal, metrics.ToD(aggr.Min), smallSkew)
		assert.InEpsilon(t, normal, metrics.ToD(aggr.Avg), smallSkew)
		assert.InEpsilon(t, normal, metrics.ToD(aggr.Max), smallSkew)
	}

	outlierTrail := skewTrail(r, simpleTrail, 2.0+smallSkew, 3.0+smallSkew)
	trails = append(trails, &outlierTrail)
	out.AddMetricSamples(trails)
	expSamples <- []Sample{
		*NewSampleFromTrail(&outlierTrail),
		{
			Type:   DataTypeAggregatedHTTPReqs,
			Metric: "http_req_li_all",
			Data: func(data interface{}) {
				aggrData, ok := data.(*SampleDataAggregatedHTTPReqs)
				assert.True(t, ok)
				assert.True(t, aggrData.Tags.IsEqual(expectedTags))
				assert.Equal(t, out.config.AggregationMinSamples.Int64, int64(aggrData.Count))
				assert.Equal(t, "aggregated_trend", aggrData.Type)
				assert.InDelta(t, now.UnixNano(), aggrData.Time*1000, float64(out.config.AggregationPeriod.Duration))

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

	require.NoError(t, out.Stop())
}

func TestCloudOutputMaxPerPacket(t *testing.T) {
	t.Parallel()
	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())
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
	tb.Mux.HandleFunc("/v1/tests/12", func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(http.StatusOK) })

	out, err := newOutput(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{"host": "%s", "noCompress": true}`, tb.ServerHTTP.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
	})
	require.NoError(t, err)
	require.NoError(t, err)
	now := time.Now()
	tags := metrics.IntoSampleTags(&map[string]string{"test": "mest", "a": "b"})
	gotTheLimit := false
	var m sync.Mutex

	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", out.referenceID),
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

	require.NoError(t, out.Start())

	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		Time:   now,
		Metric: builtinMetrics.VUs,
		Tags:   metrics.NewSampleTags(tags.CloneTags()),
		Value:  1.0,
	}})
	for j := time.Duration(1); j <= 200; j++ {
		container := make([]metrics.SampleContainer, 0, 500)
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
				Tags:         metrics.NewSampleTags(tags.CloneTags()),
			})
		}
		out.AddMetricSamples(container)
	}

	require.NoError(t, out.Stop())
	require.True(t, gotTheLimit)
}

func TestCloudOutputStopSendingMetric(t *testing.T) {
	t.Parallel()
	t.Run("stop engine on error", func(t *testing.T) {
		t.Parallel()
		testCloudOutputStopSendingMetric(t, true)
	})

	t.Run("don't stop engine on error", func(t *testing.T) {
		t.Parallel()
		testCloudOutputStopSendingMetric(t, false)
	})
}

func testCloudOutputStopSendingMetric(t *testing.T, stopOnError bool) {
	tb := httpmultibin.NewHTTPMultiBin(t)
	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())
	tb.Mux.HandleFunc("/v1/tests", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		require.NoError(t, err)
		data := &cloudapi.TestRun{}
		err = json.Unmarshal(body, &data)
		require.NoError(t, err)
		assert.Equal(t, "my-custom-name", data.Name)

		_, err = fmt.Fprint(resp, `{
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
	tb.Mux.HandleFunc("/v1/tests/12", func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(http.StatusOK) })

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{
			"host": "%s", "noCompress": true,
			"maxMetricSamplesPerPackage": 50,
			"name": "something-that-should-be-overwritten",
			"stopOnError": %t
		}`, tb.ServerHTTP.URL, stopOnError)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
			External: map[string]json.RawMessage{
				"loadimpact": json.RawMessage(`{"name": "my-custom-name"}`),
			},
		},
		ScriptPath: &url.URL{Path: "/script.js"},
	})
	var expectedEngineStopFuncCalled int64
	if stopOnError {
		expectedEngineStopFuncCalled = 1
	}
	var engineStopFuncCalled int64
	out.engineStopFunc = func(error) {
		atomic.AddInt64(&engineStopFuncCalled, 1)
	}
	require.NoError(t, err)
	now := time.Now()
	tags := metrics.IntoSampleTags(&map[string]string{"test": "mest", "a": "b"})

	count := 1
	max := 5
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", out.referenceID),
		func(w http.ResponseWriter, r *http.Request) {
			count++
			if count == max {
				type payload struct {
					Error cloudapi.ErrorResponse `json:"error"`
				}
				res := &payload{}
				res.Error = cloudapi.ErrorResponse{Code: 4}
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

	require.NoError(t, out.Start())

	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		Time:   now,
		Metric: builtinMetrics.VUs,
		Tags:   metrics.NewSampleTags(tags.CloneTags()),
		Value:  1.0,
	}})
	for j := time.Duration(1); j <= 200; j++ {
		container := make([]metrics.SampleContainer, 0, 500)
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
				Tags:         metrics.NewSampleTags(tags.CloneTags()),
			})
		}
		out.AddMetricSamples(container)
	}

	require.NoError(t, out.Stop())

	require.Equal(t, lib.RunStatusQueued, out.runStatus)
	select {
	case <-out.stopSendingMetrics:
		// all is fine
	default:
		t.Fatal("sending metrics wasn't stopped")
	}
	require.Equal(t, max, count)
	require.Equal(t, expectedEngineStopFuncCalled, engineStopFuncCalled)

	nBufferSamples := len(out.bufferSamples)
	nBufferHTTPTrails := len(out.bufferHTTPTrails)
	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		Time:   now,
		Metric: builtinMetrics.VUs,
		Tags:   metrics.NewSampleTags(tags.CloneTags()),
		Value:  1.0,
	}})
	if nBufferSamples != len(out.bufferSamples) || nBufferHTTPTrails != len(out.bufferHTTPTrails) {
		t.Errorf("Output still collects data after stop sending metrics")
	}
}

func TestCloudOutputRequireScriptName(t *testing.T) {
	t.Parallel()
	_, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: ""},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "script name not set")
}

func TestCloudOutputAggregationPeriodZeroNoBlock(t *testing.T) {
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
	tb.Mux.HandleFunc("/v1/tests/123", func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(http.StatusOK) })

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{
			"host": "%s", "noCompress": true,
			"maxMetricSamplesPerPackage": 50
		}`, tb.ServerHTTP.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
	})
	require.NoError(t, err)

	assert.True(t, out.config.Host.Valid)
	assert.Equal(t, tb.ServerHTTP.URL, out.config.Host.String)
	assert.True(t, out.config.NoCompress.Valid)
	assert.True(t, out.config.NoCompress.Bool)
	assert.False(t, out.config.MetricPushInterval.Valid)
	assert.False(t, out.config.AggregationPeriod.Valid)
	assert.False(t, out.config.AggregationWaitPeriod.Valid)

	require.NoError(t, out.Start())
	assert.Equal(t, "123", out.referenceID)
	assert.True(t, out.config.MetricPushInterval.Valid)
	assert.Equal(t, types.Duration(10*time.Millisecond), out.config.MetricPushInterval.Duration)
	assert.True(t, out.config.AggregationPeriod.Valid)
	assert.Equal(t, types.Duration(0), out.config.AggregationPeriod.Duration)
	assert.True(t, out.config.AggregationWaitPeriod.Valid)
	assert.Equal(t, types.Duration(5*time.Millisecond), out.config.AggregationWaitPeriod.Duration)

	expSamples := make(chan []Sample)
	defer close(expSamples)
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", out.referenceID), getSampleChecker(t, expSamples))

	require.NoError(t, out.Stop())
	require.Equal(t, lib.RunStatusQueued, out.runStatus)
}

func TestCloudOutputPushRefID(t *testing.T) {
	t.Parallel()
	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())
	expSamples := make(chan []Sample)
	defer close(expSamples)

	tb := httpmultibin.NewHTTPMultiBin(t)
	failHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("%s should not have been called at all", r.RequestURI)
	})
	tb.Mux.HandleFunc("/v1/tests", failHandler)
	tb.Mux.HandleFunc("/v1/tests/333", failHandler)
	tb.Mux.HandleFunc("/v1/metrics/333", getSampleChecker(t, expSamples))

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{
			"host": "%s", "noCompress": true,
			"metricPushInterval": "10ms",
			"aggregationPeriod": "0ms",
			"pushRefID": "333"
		}`, tb.ServerHTTP.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
	})
	require.NoError(t, err)

	assert.Equal(t, "333", out.config.PushRefID.String)
	require.NoError(t, out.Start())
	assert.Equal(t, "333", out.referenceID)

	now := time.Now()
	tags := metrics.IntoSampleTags(&map[string]string{"test": "mest", "a": "b"})

	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		Time:   now,
		Metric: builtinMetrics.HTTPReqDuration,
		Tags:   tags,
		Value:  123.45,
	}})
	exp := []Sample{{
		Type:   DataTypeSingle,
		Metric: metrics.HTTPReqDurationName,
		Data: &SampleDataSingle{
			Type:  builtinMetrics.HTTPReqDuration.Type,
			Time:  toMicroSecond(now),
			Tags:  tags,
			Value: 123.45,
		},
	}}

	select {
	case expSamples <- exp:
	case <-time.After(5 * time.Second):
		t.Error("test timeout")
	}

	require.NoError(t, out.Stop())
}

func TestCloudOutputRecvIterLIAllIterations(t *testing.T) {
	t.Parallel()
	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())
	tb := httpmultibin.NewHTTPMultiBin(t)
	tb.Mux.HandleFunc("/v1/tests", http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		require.NoError(t, err)
		data := &cloudapi.TestRun{}
		err = json.Unmarshal(body, &data)
		require.NoError(t, err)
		assert.Equal(t, "script.js", data.Name)

		_, err = fmt.Fprintf(resp, `{"reference_id": "123"}`)
		require.NoError(t, err)
	}))
	tb.Mux.HandleFunc("/v1/tests/123", func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(http.StatusOK) })

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{
			"host": "%s", "noCompress": true,
			"maxMetricSamplesPerPackage": 50
		}`, tb.ServerHTTP.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "path/to/script.js"},
	})
	require.NoError(t, err)

	gotIterations := false
	var m sync.Mutex
	expValues := map[string]float64{
		"data_received":      100,
		"data_sent":          200,
		"iteration_duration": 60000,
		"iterations":         1,
	}

	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", out.referenceID),
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

	require.NoError(t, out.Start())

	now := time.Now()
	simpleNetTrail := netext.NetTrail{
		BytesRead:     100,
		BytesWritten:  200,
		FullIteration: true,
		StartTime:     now.Add(-time.Minute),
		EndTime:       now,
		Samples: []metrics.Sample{
			{
				Time:   now,
				Metric: builtinMetrics.DataSent,
				Value:  float64(200),
			},
			{
				Time:   now,
				Metric: builtinMetrics.DataReceived,
				Value:  float64(100),
			},
			{
				Time:   now,
				Metric: builtinMetrics.Iterations,
				Value:  1,
			},
		},
	}

	out.AddMetricSamples([]metrics.SampleContainer{&simpleNetTrail})
	require.NoError(t, out.Stop())
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
				Opaque: "go.k6.io/k6/samples/http_get.js",
			},
			expected: "http_get.js",
		},
		{
			url:      mustParse("http://go.k6.io/k6/samples/http_get.js"),
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
			out, err := newOutput(output.Params{
				Logger: testutils.NewLogger(t),
				ScriptOptions: lib.Options{
					Duration:   types.NullDurationFrom(1 * time.Second),
					SystemTags: &metrics.DefaultSystemTagSet,
				},
				ScriptPath: testCase.url,
			})
			require.NoError(t, err)
			require.Equal(t, out.config.Name.String, testCase.expected)
		})
	}
}

func TestPublishMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g, err := gzip.NewReader(r.Body)

		require.NoError(t, err)
		var buf bytes.Buffer
		_, err = io.Copy(&buf, g) //nolint:gosec
		require.NoError(t, err)
		byteCount, err := strconv.Atoi(r.Header.Get("x-payload-byte-count"))
		require.NoError(t, err)
		require.Equal(t, buf.Len(), byteCount)

		samplesCount, err := strconv.Atoi(r.Header.Get("x-payload-sample-count"))
		require.NoError(t, err)
		var samples []*Sample
		err = json.Unmarshal(buf.Bytes(), &samples)
		require.NoError(t, err)
		require.Equal(t, len(samples), samplesCount)

		_, err = fmt.Fprintf(w, "")
		require.NoError(t, err)
	}))
	defer server.Close()

	out, err := newOutput(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{"host": "%s", "noCompress": false}`, server.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "script.js"},
	})
	require.NoError(t, err)

	samples := []*Sample{
		{
			Type:   "Point",
			Metric: "metric",
			Data: &SampleDataSingle{
				Type:  1,
				Time:  toMicroSecond(time.Now()),
				Value: 1.2,
			},
		},
	}
	err = out.client.PushMetric("1", samples)

	assert.Nil(t, err)
}

func TestNewOutputClientTimeout(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
	}))
	defer ts.Close()

	out, err := newOutput(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{"host": "%s",  "timeout": "2ms"}`, ts.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "script.js"},
	})
	require.NoError(t, err)

	err = out.client.PushMetric("testmetric", nil)
	assert.True(t, os.IsTimeout(err))
}
