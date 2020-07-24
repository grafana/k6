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
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/assert"

	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/stats"
)

func TestSampleMarshaling(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exptoMicroSecond := now.UnixNano() / 1000

	testCases := []struct {
		s    *Sample
		json string
	}{
		{
			&Sample{
				Type:   DataTypeSingle,
				Metric: metrics.VUs.Name,
				Data: &SampleDataSingle{
					Type:  metrics.VUs.Type,
					Time:  toMicroSecond(now),
					Tags:  stats.IntoSampleTags(&map[string]string{"aaa": "bbb", "ccc": "123"}),
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
					Tags: stats.IntoSampleTags(&map[string]string{"test": "mest"}),
					Values: map[string]float64{
						metrics.DataSent.Name:          1234.5,
						metrics.DataReceived.Name:      6789.1,
						metrics.IterationDuration.Name: stats.D(10 * time.Second),
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
	assert.Equal(t, m.Min, stats.D(1*time.Second))
	assert.Equal(t, m.Max, stats.D(10*time.Second))
	assert.Equal(t, m.Avg, stats.D(4*time.Second))
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
func getDurations(count int, min, multiplier float64) durations {
	data := make(durations, count)
	for j := 0; j < count; j++ {
		data[j] = time.Duration(min + rand.Float64()*multiplier)
	}
	return data
}
func BenchmarkDurationBounds(b *testing.B) {
	iqrRadius := 0.25 // If it's something different, the Q in IQR won't make much sense...
	iqrLowerCoef := 1.5
	iqrUpperCoef := 1.5

	getData := func(b *testing.B, count int) durations {
		b.StopTimer()
		defer b.StartTimer()
		return getDurations(count, 0.1*float64(time.Second), float64(time.Second))
	}

	for count := 100; count <= 5000; count += 500 {
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
	mult := time.Millisecond
	for _, count := range []int{1, 2, 3, 4, 5, 10, 15, 20, 25, 50, 100, 250 + rand.Intn(100)} {
		count := count
		t.Run(fmt.Sprintf("simple-%d", count), func(t *testing.T) {
			t.Parallel()
			data := make(durations, count)
			for i := 0; i < count; i++ {
				data[i] = time.Duration(i) * mult
			}
			rand.Shuffle(len(data), data.Swap)
			for i := 0; i < 10; i++ {
				dataCopy := make(durations, count)
				assert.Equal(t, count, copy(dataCopy, data))
				k := rand.Intn(count)
				assert.Equal(t, dataCopy.quickSelect(k), time.Duration(k)*mult)
			}
		})
		t.Run(fmt.Sprintf("random-%d", count), func(t *testing.T) {
			t.Parallel()

			testCases := []struct{ r, l, u float64 }{
				{0.25, 1.5, 1.5}, // Textbook
				{0.25, 1.5, 1.3}, // Defaults
				{0.1, 0.5, 0.3},  // Extreme narrow
				{0.3, 2, 1.8},    // Extreme wide
			}

			for tcNum, tc := range testCases {
				tc := tc
				data := getDurations(count, 0.3*float64(time.Second), 2*float64(time.Second))
				dataForSort := make(durations, count)
				dataForSelect := make(durations, count)
				assert.Equal(t, count, copy(dataForSort, data))
				assert.Equal(t, count, copy(dataForSelect, data))
				assert.Equal(t, dataForSort, dataForSelect)

				t.Run(fmt.Sprintf("bounds-tc%d", tcNum), func(t *testing.T) {
					t.Parallel()
					sortMin, sortMax := dataForSort.SortGetNormalBounds(tc.r, tc.l, tc.u, false)
					selectMin, selectMax := dataForSelect.SelectGetNormalBounds(tc.r, tc.l, tc.u)
					assert.Equal(t, sortMin, selectMin)
					assert.Equal(t, sortMax, selectMax)

					k := rand.Intn(count)
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
