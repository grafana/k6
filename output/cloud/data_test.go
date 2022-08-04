package cloud

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/metrics"
)

func TestSampleMarshaling(t *testing.T) {
	t.Parallel()

	builtinMetrics := metrics.RegisterBuiltinMetrics(metrics.NewRegistry())
	now := time.Now()
	exptoMicroSecond := now.UnixNano() / 1000

	testCases := []struct {
		s    *Sample
		json string
	}{
		{
			&Sample{
				Type:   DataTypeSingle,
				Metric: metrics.VUsName,
				Data: &SampleDataSingle{
					Type:  builtinMetrics.VUs.Type,
					Time:  toMicroSecond(now),
					Tags:  metrics.IntoSampleTags(&map[string]string{"aaa": "bbb", "ccc": "123"}),
					Value: 999,
				},
			},
			fmt.Sprintf(`{"type":"Point","metric":"vus","data":{"time":"%d","type":"gauge","tags":{"aaa":"bbb","ccc":"123"},"value":999}}`, exptoMicroSecond),
		},
		{
			&Sample{
				Type:   DataTypeMap,
				Metric: "iter_li_all",
				Data: &SampleDataMap{
					Time: toMicroSecond(now),
					Tags: metrics.IntoSampleTags(&map[string]string{"test": "mest"}),
					Values: map[string]float64{
						metrics.DataSentName:          1234.5,
						metrics.DataReceivedName:      6789.1,
						metrics.IterationDurationName: metrics.D(10 * time.Second),
					},
				},
			},
			fmt.Sprintf(`{"type":"Points","metric":"iter_li_all","data":{"time":"%d","type":"counter","tags":{"test":"mest"},"values":{"data_received":6789.1,"data_sent":1234.5,"iteration_duration":10000}}}`, exptoMicroSecond),
		},
		{
			NewSampleFromTrail(&httpext.Trail{
				EndTime:        now,
				Duration:       123000,
				Blocked:        1000,
				Connecting:     2000,
				TLSHandshaking: 3000,
				Sending:        4000,
				Waiting:        5000,
				Receiving:      6000,
			}),
			fmt.Sprintf(`{"type":"Points","metric":"http_req_li_all","data":{"time":"%d","type":"counter","values":{"http_req_blocked":0.001,"http_req_connecting":0.002,"http_req_duration":0.123,"http_req_receiving":0.006,"http_req_sending":0.004,"http_req_tls_handshaking":0.003,"http_req_waiting":0.005,"http_reqs":1}}}`, exptoMicroSecond),
		},
		{
			NewSampleFromTrail(&httpext.Trail{
				EndTime:        now,
				Duration:       123000,
				Blocked:        1000,
				Connecting:     2000,
				TLSHandshaking: 3000,
				Sending:        4000,
				Waiting:        5000,
				Receiving:      6000,
				Failed:         null.NewBool(false, true),
			}),
			fmt.Sprintf(`{"type":"Points","metric":"http_req_li_all","data":{"time":"%d","type":"counter","values":{"http_req_blocked":0.001,"http_req_connecting":0.002,"http_req_duration":0.123,"http_req_failed":0,"http_req_receiving":0.006,"http_req_sending":0.004,"http_req_tls_handshaking":0.003,"http_req_waiting":0.005,"http_reqs":1}}}`, exptoMicroSecond),
		},
		{
			func() *Sample {
				aggrData := &SampleDataAggregatedHTTPReqs{
					Time: exptoMicroSecond,
					Type: "aggregated_trend",
					Tags: metrics.IntoSampleTags(&map[string]string{"test": "mest"}),
				}
				aggrData.Add(
					&httpext.Trail{
						EndTime:        now,
						Duration:       123000,
						Blocked:        1000,
						Connecting:     2000,
						TLSHandshaking: 3000,
						Sending:        4000,
						Waiting:        5000,
						Receiving:      6000,
					},
				)

				aggrData.Add(
					&httpext.Trail{
						EndTime:        now,
						Duration:       13000,
						Blocked:        3000,
						Connecting:     1000,
						TLSHandshaking: 4000,
						Sending:        5000,
						Waiting:        8000,
						Receiving:      8000,
					},
				)
				aggrData.CalcAverages()

				return &Sample{
					Type:   DataTypeAggregatedHTTPReqs,
					Metric: "http_req_li_all",
					Data:   aggrData,
				}
			}(),
			fmt.Sprintf(`{"type":"AggregatedPoints","metric":"http_req_li_all","data":{"time":"%d","type":"aggregated_trend","count":2,"tags":{"test":"mest"},"values":{"http_req_duration":{"min":0.013,"max":0.123,"avg":0.068},"http_req_blocked":{"min":0.001,"max":0.003,"avg":0.002},"http_req_connecting":{"min":0.001,"max":0.002,"avg":0.0015},"http_req_tls_handshaking":{"min":0.003,"max":0.004,"avg":0.0035},"http_req_sending":{"min":0.004,"max":0.005,"avg":0.0045},"http_req_waiting":{"min":0.005,"max":0.008,"avg":0.0065},"http_req_receiving":{"min":0.006,"max":0.008,"avg":0.007}}}}`, exptoMicroSecond),
		},
		{
			func() *Sample {
				aggrData := &SampleDataAggregatedHTTPReqs{
					Time: exptoMicroSecond,
					Type: "aggregated_trend",
					Tags: metrics.IntoSampleTags(&map[string]string{"test": "mest"}),
				}
				aggrData.Add(
					&httpext.Trail{
						EndTime:        now,
						Duration:       123000,
						Blocked:        1000,
						Connecting:     2000,
						TLSHandshaking: 3000,
						Sending:        4000,
						Waiting:        5000,
						Receiving:      6000,
						Failed:         null.BoolFrom(false),
					},
				)

				aggrData.Add(
					&httpext.Trail{
						EndTime:        now,
						Duration:       13000,
						Blocked:        3000,
						Connecting:     1000,
						TLSHandshaking: 4000,
						Sending:        5000,
						Waiting:        8000,
						Receiving:      8000,
					},
				)
				aggrData.CalcAverages()

				return &Sample{
					Type:   DataTypeAggregatedHTTPReqs,
					Metric: "http_req_li_all",
					Data:   aggrData,
				}
			}(),
			fmt.Sprintf(`{"type":"AggregatedPoints","metric":"http_req_li_all","data":{"time":"%d","type":"aggregated_trend","count":2,"tags":{"test":"mest"},"values":{"http_req_duration":{"min":0.013,"max":0.123,"avg":0.068},"http_req_blocked":{"min":0.001,"max":0.003,"avg":0.002},"http_req_connecting":{"min":0.001,"max":0.002,"avg":0.0015},"http_req_tls_handshaking":{"min":0.003,"max":0.004,"avg":0.0035},"http_req_sending":{"min":0.004,"max":0.005,"avg":0.0045},"http_req_waiting":{"min":0.005,"max":0.008,"avg":0.0065},"http_req_receiving":{"min":0.006,"max":0.008,"avg":0.007},"http_req_failed":{"count":1,"nz_count":0}}}}`, exptoMicroSecond),
		},
	}

	for _, tc := range testCases {
		sJSON, err := easyjson.Marshal(tc.s)
		if !assert.NoError(t, err) {
			continue
		}
		t.Logf(string(sJSON))
		assert.JSONEq(t, tc.json, string(sJSON))

		var newS Sample
		assert.NoError(t, json.Unmarshal(sJSON, &newS))
		assert.Equal(t, tc.s.Type, newS.Type)
		assert.Equal(t, tc.s.Metric, newS.Metric)
		assert.IsType(t, tc.s.Data, newS.Data)
		// Cannot directly compare tc.s.Data and newS.Data (because of internal time.Time and SampleTags fields)
		newJSON, err := easyjson.Marshal(newS)
		assert.NoError(t, err)
		assert.JSONEq(t, string(sJSON), string(newJSON))
	}
}

func TestMetricAggregation(t *testing.T) {
	m := AggregatedMetric{}
	m.Add(1 * time.Second)
	m.Add(1 * time.Second)
	m.Add(3 * time.Second)
	m.Add(5 * time.Second)
	m.Add(10 * time.Second)
	m.Calc(5)
	assert.Equal(t, m.Min, metrics.D(1*time.Second))
	assert.Equal(t, m.Max, metrics.D(10*time.Second))
	assert.Equal(t, m.Avg, metrics.D(4*time.Second))
}

// For more realistic request time distributions, import
// "gonum.org/v1/gonum/stat/distuv" and use something like this:
//
// randSrc := rand.NewSource(uint64(time.Now().UnixNano()))
// dist := distuv.LogNormal{Mu: 0, Sigma: 0.5, Src: randSrc}
//
// then set the data elements to time.Duration(dist.Rand() * multiplier)
//
// I've not used that after the initial tests because it's a big
// external dependency that's not really needed for the tests at
// this point.
func getDurations(r *rand.Rand, count int, min, multiplier float64) durations {
	data := make(durations, count)
	for j := 0; j < count; j++ {
		data[j] = time.Duration(min + r.Float64()*multiplier) //nolint:gosec
	}
	return data
}

func BenchmarkDurationBounds(b *testing.B) {
	iqrRadius := 0.25 // If it's something different, the Q in IQR won't make much sense...
	iqrLowerCoef := 1.5
	iqrUpperCoef := 1.5

	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed)) //nolint:gosec
	b.Logf("Random source seeded with %d\n", seed)

	getData := func(b *testing.B, count int) durations {
		b.StopTimer()
		defer b.StartTimer()
		return getDurations(r, count, 0.1*float64(time.Second), float64(time.Second))
	}

	for count := 100; count <= 5000; count += 500 {
		count := count
		b.Run(fmt.Sprintf("Sort-no-interp-%d-elements", count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				data := getData(b, count)
				data.SortGetNormalBounds(iqrRadius, iqrLowerCoef, iqrUpperCoef, false)
			}
		})
		b.Run(fmt.Sprintf("Sort-with-interp-%d-elements", count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				data := getData(b, count)
				data.SortGetNormalBounds(iqrRadius, iqrLowerCoef, iqrUpperCoef, true)
			}
		})
		b.Run(fmt.Sprintf("Select-%d-elements", count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				data := getData(b, count)
				data.SelectGetNormalBounds(iqrRadius, iqrLowerCoef, iqrUpperCoef)
			}
		})
	}
}

func TestQuickSelectAndBounds(t *testing.T) {
	t.Parallel()

	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed)) //nolint:gosec
	t.Logf("Random source seeded with %d\n", seed)

	mult := time.Millisecond
	for _, count := range []int{1, 2, 3, 4, 5, 10, 15, 20, 25, 50, 100, 250 + r.Intn(100)} {
		count := count
		t.Run(fmt.Sprintf("simple-%d", count), func(t *testing.T) {
			data := make(durations, count)
			for i := 0; i < count; i++ {
				data[i] = time.Duration(i) * mult
			}
			rand.Shuffle(len(data), data.Swap)
			for i := 0; i < 10; i++ {
				dataCopy := make(durations, count)
				assert.Equal(t, count, copy(dataCopy, data))
				k := r.Intn(count)
				assert.Equal(t, dataCopy.quickSelect(k), time.Duration(k)*mult)
			}
		})
		t.Run(fmt.Sprintf("random-%d", count), func(t *testing.T) {
			testCases := []struct{ r, l, u float64 }{
				{0.25, 1.5, 1.5}, // Textbook
				{0.25, 1.5, 1.3}, // Defaults
				{0.1, 0.5, 0.3},  // Extreme narrow
				{0.3, 2, 1.8},    // Extreme wide
			}

			for tcNum, tc := range testCases {
				tc := tc
				data := getDurations(r, count, 0.3*float64(time.Second), 2*float64(time.Second))
				dataForSort := make(durations, count)
				dataForSelect := make(durations, count)
				assert.Equal(t, count, copy(dataForSort, data))
				assert.Equal(t, count, copy(dataForSelect, data))
				assert.Equal(t, dataForSort, dataForSelect)

				t.Run(fmt.Sprintf("bounds-tc%d", tcNum), func(t *testing.T) {
					sortMin, sortMax := dataForSort.SortGetNormalBounds(tc.r, tc.l, tc.u, false)
					selectMin, selectMax := dataForSelect.SelectGetNormalBounds(tc.r, tc.l, tc.u)
					assert.Equal(t, sortMin, selectMin)
					assert.Equal(t, sortMax, selectMax)

					k := r.Intn(count)
					assert.Equal(t, dataForSort[k], dataForSelect.quickSelect(k))
					assert.Equal(t, dataForSort[k], data.quickSelect(k))
				})
			}
		})
	}
}

func TestSortInterpolation(t *testing.T) {
	t.Parallel()

	// Super contrived example to make the checks easy - 11 values from 0 to 10 seconds inclusive
	count := 11
	data := make(durations, count)
	for i := 0; i < count; i++ {
		data[i] = time.Duration(i) * time.Second
	}

	min, max := data.SortGetNormalBounds(0.25, 1, 1, true)
	// Expected values: Q1=2.5, Q3=7.5, IQR=5, so with 1 for coefficients we can expect min=-2,5, max=12.5 seconds
	assert.Equal(t, min, -2500*time.Millisecond)
	assert.Equal(t, max, 12500*time.Millisecond)
}
