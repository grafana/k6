package cloud

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
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
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

func tagEqual(expected, got json.RawMessage) bool {
	var expectedMap, gotMap map[string]string
	err := json.Unmarshal(expected, &expectedMap)
	if err != nil {
		panic("tagEqual: " + err.Error())
	}

	err = json.Unmarshal(got, &gotMap)
	if err != nil {
		panic("tagEqual: " + err.Error())
	}

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
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		receivedSamples := []Sample{}
		assert.NoError(t, json.Unmarshal(body, &receivedSamples))

		expSamples := <-expSamples
		require.Len(t, receivedSamples, len(expSamples))

		for i, expSample := range expSamples {
			receivedSample := receivedSamples[i]
			assert.Equal(t, expSample.Metric, receivedSample.Metric)
			assert.Equal(t, expSample.Type, receivedSample.Type)

			if callbackCheck, ok := expSample.Data.(func(interface{})); ok {
				callbackCheck(receivedSample.Data)
				continue
			}

			require.IsType(t, expSample.Data, receivedSample.Data)

			switch expData := expSample.Data.(type) {
			case *SampleDataSingle:
				receivedData, ok := receivedSample.Data.(*SampleDataSingle)
				assert.True(t, ok)
				assert.JSONEq(t, string(expData.Tags), string(receivedData.Tags))
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
				assert.JSONEq(t, string(expData.Tags), string(receivedData.Tags))
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
		tcNum, minSamples := tcNum, minSamples
		t.Run(fmt.Sprintf("tc%d_minSamples%d", tcNum, minSamples), func(t *testing.T) {
			t.Parallel()
			getTestRunner(minSamples)
		})
	}
}

func runCloudOutputTestCase(t *testing.T, minSamples int) {
	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed)) //nolint:gosec
	t.Logf("Random source seeded with %d\n", seed)

	tb := httpmultibin.NewHTTPMultiBin(t)
	registry := metrics.NewRegistry()

	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	out, err := newTestOutput(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{"host": "%s", "noCompress": true}`, tb.ServerHTTP.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Environment: map[string]string{
			"K6_CLOUD_PUSH_REF_ID":               "123",
			"K6_CLOUD_METRIC_PUSH_INTERVAL":      "10ms",
			"K6_CLOUD_AGGREGATION_PERIOD":        "30ms",
			"K6_CLOUD_AGGREGATION_CALC_INTERVAL": "40ms",
			"K6_CLOUD_AGGREGATION_WAIT_PERIOD":   "5ms",
			"K6_CLOUD_AGGREGATION_MIN_SAMPLES":   strconv.Itoa(minSamples),
		},
		ScriptPath: &url.URL{Path: "/script.js"},
	})
	require.NoError(t, err)

	out.SetTestRunID("123")
	require.NoError(t, out.Start())

	now := time.Now()
	tagMap := map[string]string{"test": "mest", "a": "b", "name": "name", "url": "name"}
	tags := registry.RootTagSet().WithTagsFromMap(tagMap)

	expSamples := make(chan []Sample)
	defer close(expSamples)
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", out.referenceID), getSampleChecker(t, expSamples))

	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: builtinMetrics.VUs,
			Tags:   tags,
		},
		Time:  now,
		Value: 1.0,
	}})

	enctags, err := json.Marshal(tags)
	require.NoError(t, err)
	expSamples <- []Sample{{
		Type:   DataTypeSingle,
		Metric: metrics.VUsName,
		Data: &SampleDataSingle{
			Type:  builtinMetrics.VUs.Type,
			Time:  toMicroSecond(now),
			Tags:  enctags,
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
	durations := make([]time.Duration, 0, len(trails))
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
				assert.JSONEq(t, `{"test": "mest", "a": "b", "name": "name", "url": "name"}`, string(aggrData.Tags))
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

	require.NoError(t, out.StopWithTestError(nil))
}

func TestCloudOutputMaxPerPacket(t *testing.T) {
	t.Parallel()

	tb := httpmultibin.NewHTTPMultiBin(t)
	out, err := newTestOutput(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{"host": "%s", "noCompress": true}`, tb.ServerHTTP.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
	})
	require.NoError(t, err)
	out.SetTestRunID("12")

	maxMetricSamplesPerPackage := 20
	out.config.MaxMetricSamplesPerPackage = null.IntFrom(int64(maxMetricSamplesPerPackage))

	now := time.Now()
	registry := metrics.NewRegistry()
	tags := registry.RootTagSet().WithTagsFromMap(map[string]string{"test": "mest", "a": "b"})
	gotTheLimit := false
	var m sync.Mutex
	tb.Mux.HandleFunc(fmt.Sprintf("/v1/metrics/%s", out.referenceID),
		func(_ http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
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

	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: builtinMetrics.VUs,
			Tags:   tags,
		},
		Time:  now,
		Value: 1.0,
	}})
	for j := time.Duration(1); j <= 200; j++ {
		container := make([]metrics.SampleContainer, 0, 500)
		for i := time.Duration(1); i <= 50; i++ {
			//nolint:durationcheck
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
				Tags:         tags,
			})
		}
		out.AddMetricSamples(container)
	}

	require.NoError(t, out.StopWithTestError(nil))
	assert.True(t, gotTheLimit)
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
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())

	tb := httpmultibin.NewHTTPMultiBin(t)
	out, err := newTestOutput(output.Params{
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
	var expectedTestStopFuncCalled int64
	if stopOnError {
		expectedTestStopFuncCalled = 1
	}
	var TestStopFuncCalled int64
	out.testStopFunc = func(error) {
		atomic.AddInt64(&TestStopFuncCalled, 1)
	}
	require.NoError(t, err)
	now := time.Now()
	tags := registry.RootTagSet().WithTagsFromMap(map[string]string{"test": "mest", "a": "b"})

	count := 1
	max := 5
	tb.Mux.HandleFunc("/v1/metrics/12", func(w http.ResponseWriter, r *http.Request) {
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
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		receivedSamples := []Sample{}
		assert.NoError(t, json.Unmarshal(body, &receivedSamples))
	})

	out.SetTestRunID("12")
	require.NoError(t, out.Start())

	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: builtinMetrics.VUs,
			Tags:   tags,
		},
		Time:  now,
		Value: 1.0,
	}})
	for j := time.Duration(1); j <= 200; j++ {
		container := make([]metrics.SampleContainer, 0, 500)
		for i := time.Duration(1); i <= 50; i++ {
			//nolint:durationcheck
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
				Tags:         tags,
			})
		}
		out.AddMetricSamples(container)
	}

	require.NoError(t, out.StopWithTestError(nil))

	select {
	case <-out.stopSendingMetrics:
		// all is fine
	default:
		t.Fatal("sending metrics wasn't stopped")
	}
	require.Equal(t, max, count)
	require.Equal(t, expectedTestStopFuncCalled, TestStopFuncCalled)

	nBufferSamples := len(out.bufferSamples)
	nBufferHTTPTrails := len(out.bufferHTTPTrails)
	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: builtinMetrics.VUs,
			Tags:   tags,
		},
		Time:  now,
		Value: 1.0,
	}})
	if nBufferSamples != len(out.bufferSamples) || nBufferHTTPTrails != len(out.bufferHTTPTrails) {
		t.Errorf("Output still collects data after stop sending metrics")
	}
}

func TestCloudOutputPushRefID(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
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

	out, err := newTestOutput(output.Params{
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

	out.SetTestRunID("333")
	require.NoError(t, out.Start())

	now := time.Now()
	tags := registry.RootTagSet().WithTagsFromMap(map[string]string{"test": "mest", "a": "b"})
	encodedTags, err := json.Marshal(tags)
	require.NoError(t, err)

	out.AddMetricSamples([]metrics.SampleContainer{metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: builtinMetrics.HTTPReqDuration,
			Tags:   tags,
		},
		Time:  now,
		Value: 123.45,
	}})
	exp := []Sample{{
		Type:   DataTypeSingle,
		Metric: metrics.HTTPReqDurationName,
		Data: &SampleDataSingle{
			Type:  builtinMetrics.HTTPReqDuration.Type,
			Time:  toMicroSecond(now),
			Tags:  encodedTags,
			Value: 123.45,
		},
	}}

	select {
	case expSamples <- exp:
	case <-time.After(5 * time.Second):
		t.Error("test timeout")
	}

	require.NoError(t, out.StopWithTestError(nil))
}

func TestCloudOutputRecvIterLIAllIterations(t *testing.T) {
	t.Parallel()

	tb := httpmultibin.NewHTTPMultiBin(t)
	out, err := newTestOutput(output.Params{
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

	tb.Mux.HandleFunc("/v1/metrics/123", func(_ http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
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

	out.SetTestRunID("123")
	require.NoError(t, out.Start())

	now := time.Now()
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	simpleNetTrail := netext.NetTrail{
		BytesRead:     100,
		BytesWritten:  200,
		FullIteration: true,
		StartTime:     now.Add(-time.Minute),
		EndTime:       now,
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: builtinMetrics.DataSent,
					Tags:   registry.RootTagSet(),
				},
				Time:  now,
				Value: float64(200),
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: builtinMetrics.DataReceived,
					Tags:   registry.RootTagSet(),
				},
				Time:  now,
				Value: float64(100),
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: builtinMetrics.Iterations,
					Tags:   registry.RootTagSet(),
				},
				Time:  now,
				Value: 1,
			},
		},
	}

	out.AddMetricSamples([]metrics.SampleContainer{&simpleNetTrail})
	require.NoError(t, out.StopWithTestError(nil))
	require.True(t, gotIterations)
}

func TestPublishMetric(t *testing.T) {
	t.Parallel()
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

	out, err := newTestOutput(output.Params{
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
	assert.NoError(t, err)
}

func TestNewOutputClientTimeout(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer ts.Close()

	out, err := newTestOutput(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: json.RawMessage(fmt.Sprintf(`{"host": "%s",  "timeout": "2ms"}`, ts.URL)),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(50 * time.Millisecond),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "script.js"},
	})
	require.NoError(t, err)

	err = out.client.PushMetric("testmetric", nil)
	assert.True(t, os.IsTimeout(err)) //nolint:forbidigo
}

func newTestOutput(params output.Params) (*Output, error) {
	conf, err := cloudapi.GetConsolidatedConfig(
		params.JSONConfig, params.Environment, params.ConfigArgument, params.ScriptOptions.External)
	if err != nil {
		return nil, err
	}

	apiClient := cloudapi.NewClient(
		params.Logger, conf.Token.String, conf.Host.String,
		consts.Version, conf.Timeout.TimeDuration())

	return New(params.Logger, conf, apiClient)
}
