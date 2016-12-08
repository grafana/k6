/*

k6 - a next-generation load testing tool
Copyright (C) 2016 Load Impact

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.

*/

package stats

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMetricHumanizeValue(t *testing.T) {
	data := map[*Metric]map[float64]string{
		&Metric{Type: Counter, Contains: Default}: map[float64]string{
			1.0:     "1",
			1.5:     "1.5",
			1.54321: "1.54321",
		},
		&Metric{Type: Gauge, Contains: Default}: map[float64]string{
			1.0:     "1",
			1.5:     "1.5",
			1.54321: "1.54321",
		},
		&Metric{Type: Trend, Contains: Default}: map[float64]string{
			1.0:     "1",
			1.5:     "1.5",
			1.54321: "1.54321",
		},
		&Metric{Type: Counter, Contains: Time}: map[float64]string{
			float64(1):               "1ns",
			float64(12):              "12ns",
			float64(123):             "123ns",
			float64(1234):            "1.23µs",
			float64(12345):           "12.34µs",
			float64(123456):          "123.45µs",
			float64(1234567):         "1.23ms",
			float64(12345678):        "12.34ms",
			float64(123456789):       "123.45ms",
			float64(1234567890):      "1.23s",
			float64(12345678901):     "12.34s",
			float64(123456789012):    "2m3s",
			float64(1234567890123):   "20m34s",
			float64(12345678901234):  "3h25m45s",
			float64(123456789012345): "34h17m36s",
		},
		&Metric{Type: Gauge, Contains: Time}: map[float64]string{
			float64(1):               "1ns",
			float64(12):              "12ns",
			float64(123):             "123ns",
			float64(1234):            "1.23µs",
			float64(12345):           "12.34µs",
			float64(123456):          "123.45µs",
			float64(1234567):         "1.23ms",
			float64(12345678):        "12.34ms",
			float64(123456789):       "123.45ms",
			float64(1234567890):      "1.23s",
			float64(12345678901):     "12.34s",
			float64(123456789012):    "2m3s",
			float64(1234567890123):   "20m34s",
			float64(12345678901234):  "3h25m45s",
			float64(123456789012345): "34h17m36s",
		},
		&Metric{Type: Trend, Contains: Time}: map[float64]string{
			float64(1):               "1ns",
			float64(12):              "12ns",
			float64(123):             "123ns",
			float64(1234):            "1.23µs",
			float64(12345):           "12.34µs",
			float64(123456):          "123.45µs",
			float64(1234567):         "1.23ms",
			float64(12345678):        "12.34ms",
			float64(123456789):       "123.45ms",
			float64(1234567890):      "1.23s",
			float64(12345678901):     "12.34s",
			float64(123456789012):    "2m3s",
			float64(1234567890123):   "20m34s",
			float64(12345678901234):  "3h25m45s",
			float64(123456789012345): "34h17m36s",
		},
		&Metric{Type: Rate, Contains: Default}: map[float64]string{
			0.0:      "0.00%",
			0.01:     "1.00%",
			0.02:     "2.00%",
			0.022:    "2.20%",
			0.0222:   "2.22%",
			0.02222:  "2.22%",
			0.022222: "2.22%",
			0.5:      "50.00%",
			0.55:     "55.00%",
			0.555:    "55.50%",
			0.5555:   "55.55%",
			0.55555:  "55.55%",
			0.75:     "75.00%",
			1.0:      "100.00%",
			1.5:      "150.00%",
		},
	}

	for m, values := range data {
		t.Run(fmt.Sprintf("type=%s,contains=%s", m.Type.String(), m.Contains.String()), func(t *testing.T) {
			for v, s := range values {
				t.Run(fmt.Sprintf("v=%f", v), func(t *testing.T) {
					assert.Equal(t, s, m.HumanizeValue(v))
				})
			}
		})
	}
}
