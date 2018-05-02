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
	"testing"
	"time"

	"github.com/loadimpact/k6/lib/netext"

	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimestampMarshaling(t *testing.T) {
	t.Parallel()

	oldTimeFormat, err := time.Parse(
		time.RFC3339,
		//1521806137415652223 as a unix nanosecond timestamp
		"2018-03-23T13:55:37.415652223+02:00",
	)
	require.NoError(t, err)

	testCases := []struct {
		t   time.Time
		exp string
	}{
		{oldTimeFormat, `"1521806137415652"`},
		{time.Unix(1521806137, 415652223), `"1521806137415652"`},
		{time.Unix(1521806137, 0), `"1521806137000000"`},
		{time.Unix(0, 0), `"0"`},
		{time.Unix(0, 1), `"0"`},
		{time.Unix(0, 1000), `"1"`},
		{time.Unix(1, 0), `"1000000"`},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test #%d", i), func(t *testing.T) {
			res, err := json.Marshal(Timestamp(tc.t))
			require.NoError(t, err)
			assert.Equal(t, string(res), tc.exp)

			var rev Timestamp
			require.NoError(t, json.Unmarshal(res, &rev))
			diff := tc.t.Sub(time.Time(rev))
			if diff < -time.Microsecond || diff > time.Microsecond {
				t.Errorf(
					"Expected the difference to be under a microsecond, but is %s (%d and %d)",
					diff,
					tc.t.UnixNano(),
					time.Time(rev).UnixNano(),
				)
			}
		})
	}
}

func TestSampleMarshaling(t *testing.T) {
	t.Parallel()

	now := time.Now()
	expTimestamp := now.UnixNano() / 1000

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
					Time:  Timestamp(now),
					Tags:  stats.IntoSampleTags(&map[string]string{"aaa": "bbb", "ccc": "123"}),
					Value: 999,
				},
			},
			fmt.Sprintf(`{"type":"Point","metric":"vus","data":{"time":"%d","type":"gauge","tags":{"aaa":"bbb","ccc":"123"},"value":999}}`, expTimestamp),
		},
		{
			&Sample{
				Type:   DataTypeMap,
				Metric: "iter_li_all",
				Data: &SampleDataMap{
					Time: Timestamp(now),
					Tags: stats.IntoSampleTags(&map[string]string{"test": "mest"}),
					Values: map[string]float64{
						metrics.DataSent.Name:          1234.5,
						metrics.DataReceived.Name:      6789.1,
						metrics.IterationDuration.Name: stats.D(10 * time.Second),
					},
				},
			},
			fmt.Sprintf(`{"type":"Points","metric":"iter_li_all","data":{"time":"%d","type":"counter","tags":{"test":"mest"},"values":{"data_received":6789.1,"data_sent":1234.5,"iteration_duration":10000}}}`, expTimestamp),
		},
		{
			NewSampleFromTrail(&netext.Trail{
				EndTime:        now,
				Duration:       123000,
				Blocked:        1000,
				Connecting:     2000,
				TLSHandshaking: 3000,
				Sending:        4000,
				Waiting:        5000,
				Receiving:      6000,
			}),
			fmt.Sprintf(`{"type":"Points","metric":"http_req_li_all","data":{"time":"%d","type":"counter","values":{"http_req_blocked":0.001,"http_req_connecting":0.002,"http_req_duration":0.123,"http_req_receiving":0.006,"http_req_sending":0.004,"http_req_tls_handshaking":0.003,"http_req_waiting":0.005,"http_reqs":1}}}`, expTimestamp),
		},
	}

	for _, tc := range testCases {
		sJSON, err := json.Marshal(tc.s)
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
		newJSON, err := json.Marshal(newS)
		assert.NoError(t, err)
		assert.JSONEq(t, string(sJSON), string(newJSON))
	}
}
