/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package stats

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testmetric struct {
	result      string
	fixTimeUnit string
}

func TestMetricHumanizeValue(t *testing.T) {
	data := map[*Metric]map[float64]testmetric{
		{Type: Counter, Contains: Default}: {
			1.0:     testmetric{"1", "1"},
			1.5:     testmetric{"1.5", "1.5"},
			1.54321: testmetric{"1.54321", "1.54321"},
		},
		{Type: Gauge, Contains: Default}: {
			1.0:     testmetric{"1", "1"},
			1.5:     testmetric{"1.5", "1.5"},
			1.54321: testmetric{"1.54321", "1.54321"},
		},
		{Type: Trend, Contains: Default}: {
			1.0:     testmetric{"1", "1"},
			1.5:     testmetric{"1.5", "1.5"},
			1.54321: testmetric{"1.54321", "1.54321"},
		},
		{Type: Counter, Contains: Time}: {
			D(1):               testmetric{"1ns", "0.000ms"},
			D(12):              testmetric{"12ns", "0.000ms"},
			D(123):             testmetric{"123ns", "0.000ms"},
			D(1234):            testmetric{"1.23µs", "0.001ms"},
			D(12345):           testmetric{"12.34µs", "0.012ms"},
			D(123456):          testmetric{"123.45µs", "0.123ms"},
			D(1234567):         testmetric{"1.23ms", "1.235ms"},
			D(12345678):        testmetric{"12.34ms", "12.346ms"},
			D(123456789):       testmetric{"123.45ms", "123.457ms"},
			D(1234567890):      testmetric{"1.23s", "1234.568ms"},
			D(12345678901):     testmetric{"12.34s", "12345.679ms"},
			D(123456789012):    testmetric{"2m3s", "123456.789ms"},
			D(1234567890123):   testmetric{"20m34s", "1234567.890ms"},
			D(12345678901234):  testmetric{"3h25m45s", "12345678.901ms"},
			D(123456789012345): testmetric{"34h17m36s", "123456789.012ms"},
		},
		{Type: Gauge, Contains: Time}: {
			D(1):               testmetric{"1ns", "0.000ms"},
			D(12):              testmetric{"12ns", "0.000ms"},
			D(123):             testmetric{"123ns", "0.000ms"},
			D(1234):            testmetric{"1.23µs", "0.001ms"},
			D(12345):           testmetric{"12.34µs", "0.012ms"},
			D(123456):          testmetric{"123.45µs", "0.123ms"},
			D(1234567):         testmetric{"1.23ms", "1.235ms"},
			D(12345678):        testmetric{"12.34ms", "12.346ms"},
			D(123456789):       testmetric{"123.45ms", "123.457ms"},
			D(1234567890):      testmetric{"1.23s", "1234.568ms"},
			D(12345678901):     testmetric{"12.34s", "12345.679ms"},
			D(123456789012):    testmetric{"2m3s", "123456.789ms"},
			D(1234567890123):   testmetric{"20m34s", "1234567.890ms"},
			D(12345678901234):  testmetric{"3h25m45s", "12345678.901ms"},
			D(123456789012345): testmetric{"34h17m36s", "123456789.012ms"},
		},
		{Type: Trend, Contains: Time}: {
			D(1):               testmetric{"1ns", "0.000ms"},
			D(12):              testmetric{"12ns", "0.000ms"},
			D(123):             testmetric{"123ns", "0.000ms"},
			D(1234):            testmetric{"1.23µs", "0.001ms"},
			D(12345):           testmetric{"12.34µs", "0.012ms"},
			D(123456):          testmetric{"123.45µs", "0.123ms"},
			D(1234567):         testmetric{"1.23ms", "1.235ms"},
			D(12345678):        testmetric{"12.34ms", "12.346ms"},
			D(123456789):       testmetric{"123.45ms", "123.457ms"},
			D(1234567890):      testmetric{"1.23s", "1234.568ms"},
			D(12345678901):     testmetric{"12.34s", "12345.679ms"},
			D(123456789012):    testmetric{"2m3s", "123456.789ms"},
			D(1234567890123):   testmetric{"20m34s", "1234567.890ms"},
			D(12345678901234):  testmetric{"3h25m45s", "12345678.901ms"},
			D(123456789012345): testmetric{"34h17m36s", "123456789.012ms"},
		},
		{Type: Rate, Contains: Default}: {
			0.0:      testmetric{"0.00%", "0.00%"},
			0.01:     testmetric{"1.00%", "1.00%"},
			0.02:     testmetric{"2.00%", "2.00%"},
			0.022:    testmetric{"2.20%", "2.20%"},
			0.0222:   testmetric{"2.22%", "2.22%"},
			0.02222:  testmetric{"2.22%", "2.22%"},
			0.022222: testmetric{"2.22%", "2.22%"},
			0.5:      testmetric{"50.00%", "50.00%"},
			0.55:     testmetric{"55.00%", "55.00%"},
			0.555:    testmetric{"55.50%", "55.50%"},
			0.5555:   testmetric{"55.55%", "55.55%"},
			0.55555:  testmetric{"55.55%", "55.55%"},
			0.75:     testmetric{"75.00%", "75.00%"},
			1.0:      testmetric{"100.00%", "100.00%"},
			1.5:      testmetric{"150.00%", "150.00%"},
		},
	}

	for m, values := range data {
		t.Run(fmt.Sprintf("type=%s,contains=%s", m.Type.String(), m.Contains.String()), func(t *testing.T) {
			for v, mt := range values {
				t.Run(fmt.Sprintf("v=%f", v), func(t *testing.T) {
					assert.Equal(t, mt.result, m.HumanizeValue(v, false))
					assert.Equal(t, mt.fixTimeUnit, m.HumanizeValue(v, true))
				})
			}
		})
	}
}

func TestNew(t *testing.T) {
	testdata := map[string]struct {
		Type     MetricType
		SinkType Sink
	}{
		"Counter": {Counter, &CounterSink{}},
		"Gauge":   {Gauge, &GaugeSink{}},
		"Trend":   {Trend, &TrendSink{}},
		"Rate":    {Rate, &RateSink{}},
	}

	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			m := New("my_metric", data.Type)
			assert.Equal(t, "my_metric", m.Name)
			assert.IsType(t, data.SinkType, m.Sink)
		})
	}
}

func TestNewSubmetric(t *testing.T) {
	testdata := map[string]struct {
		parent string
		tags   map[string]string
	}{
		"my_metric":                 {"my_metric", nil},
		"my_metric{}":               {"my_metric", map[string]string{}},
		"my_metric{a}":              {"my_metric", map[string]string{"a": ""}},
		"my_metric{a:1}":            {"my_metric", map[string]string{"a": "1"}},
		"my_metric{ a : 1 }":        {"my_metric", map[string]string{"a": "1"}},
		"my_metric{a,b}":            {"my_metric", map[string]string{"a": "", "b": ""}},
		"my_metric{a:1,b:2}":        {"my_metric", map[string]string{"a": "1", "b": "2"}},
		"my_metric{ a : 1, b : 2 }": {"my_metric", map[string]string{"a": "1", "b": "2"}},
	}

	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			parent, sm := NewSubmetric(name)
			assert.Equal(t, data.parent, parent)
			if data.tags != nil {
				assert.EqualValues(t, data.tags, sm.Tags)
			} else {
				assert.Nil(t, sm.Tags)
			}
		})
	}
}
