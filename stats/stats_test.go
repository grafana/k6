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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetricHumanizeValue(t *testing.T) {
	t.Parallel()
	data := map[*Metric]map[float64][]string{
		{Type: Counter, Contains: Default}: {
			1.0:     {"1", "1", "1", "1"},
			1.5:     {"1.5", "1.5", "1.5", "1.5"},
			1.54321: {"1.54321", "1.54321", "1.54321", "1.54321"},
		},
		{Type: Gauge, Contains: Default}: {
			1.0:     {"1", "1", "1", "1"},
			1.5:     {"1.5", "1.5", "1.5", "1.5"},
			1.54321: {"1.54321", "1.54321", "1.54321", "1.54321"},
		},
		{Type: Trend, Contains: Default}: {
			1.0:     {"1", "1", "1", "1"},
			1.5:     {"1.5", "1.5", "1.5", "1.5"},
			1.54321: {"1.54321", "1.54321", "1.54321", "1.54321"},
		},
		{Type: Counter, Contains: Time}: {
			D(1):               {"1ns", "0.00s", "0.00ms", "0.00µs"},
			D(12):              {"12ns", "0.00s", "0.00ms", "0.01µs"},
			D(123):             {"123ns", "0.00s", "0.00ms", "0.12µs"},
			D(1234):            {"1.23µs", "0.00s", "0.00ms", "1.23µs"},
			D(12345):           {"12.34µs", "0.00s", "0.01ms", "12.35µs"},
			D(123456):          {"123.45µs", "0.00s", "0.12ms", "123.46µs"},
			D(1234567):         {"1.23ms", "0.00s", "1.23ms", "1234.57µs"},
			D(12345678):        {"12.34ms", "0.01s", "12.35ms", "12345.68µs"},
			D(123456789):       {"123.45ms", "0.12s", "123.46ms", "123456.79µs"},
			D(1234567890):      {"1.23s", "1.23s", "1234.57ms", "1234567.89µs"},
			D(12345678901):     {"12.34s", "12.35s", "12345.68ms", "12345678.90µs"},
			D(123456789012):    {"2m3s", "123.46s", "123456.79ms", "123456789.01µs"},
			D(1234567890123):   {"20m34s", "1234.57s", "1234567.89ms", "1234567890.12µs"},
			D(12345678901234):  {"3h25m45s", "12345.68s", "12345678.90ms", "12345678901.23µs"},
			D(123456789012345): {"34h17m36s", "123456.79s", "123456789.01ms", "123456789012.35µs"},
		},
		{Type: Gauge, Contains: Time}: {
			D(1):               {"1ns", "0.00s", "0.00ms", "0.00µs"},
			D(12):              {"12ns", "0.00s", "0.00ms", "0.01µs"},
			D(123):             {"123ns", "0.00s", "0.00ms", "0.12µs"},
			D(1234):            {"1.23µs", "0.00s", "0.00ms", "1.23µs"},
			D(12345):           {"12.34µs", "0.00s", "0.01ms", "12.35µs"},
			D(123456):          {"123.45µs", "0.00s", "0.12ms", "123.46µs"},
			D(1234567):         {"1.23ms", "0.00s", "1.23ms", "1234.57µs"},
			D(12345678):        {"12.34ms", "0.01s", "12.35ms", "12345.68µs"},
			D(123456789):       {"123.45ms", "0.12s", "123.46ms", "123456.79µs"},
			D(1234567890):      {"1.23s", "1.23s", "1234.57ms", "1234567.89µs"},
			D(12345678901):     {"12.34s", "12.35s", "12345.68ms", "12345678.90µs"},
			D(123456789012):    {"2m3s", "123.46s", "123456.79ms", "123456789.01µs"},
			D(1234567890123):   {"20m34s", "1234.57s", "1234567.89ms", "1234567890.12µs"},
			D(12345678901234):  {"3h25m45s", "12345.68s", "12345678.90ms", "12345678901.23µs"},
			D(123456789012345): {"34h17m36s", "123456.79s", "123456789.01ms", "123456789012.35µs"},
		},
		{Type: Trend, Contains: Time}: {
			D(1):               {"1ns", "0.00s", "0.00ms", "0.00µs"},
			D(12):              {"12ns", "0.00s", "0.00ms", "0.01µs"},
			D(123):             {"123ns", "0.00s", "0.00ms", "0.12µs"},
			D(1234):            {"1.23µs", "0.00s", "0.00ms", "1.23µs"},
			D(12345):           {"12.34µs", "0.00s", "0.01ms", "12.35µs"},
			D(123456):          {"123.45µs", "0.00s", "0.12ms", "123.46µs"},
			D(1234567):         {"1.23ms", "0.00s", "1.23ms", "1234.57µs"},
			D(12345678):        {"12.34ms", "0.01s", "12.35ms", "12345.68µs"},
			D(123456789):       {"123.45ms", "0.12s", "123.46ms", "123456.79µs"},
			D(1234567890):      {"1.23s", "1.23s", "1234.57ms", "1234567.89µs"},
			D(12345678901):     {"12.34s", "12.35s", "12345.68ms", "12345678.90µs"},
			D(123456789012):    {"2m3s", "123.46s", "123456.79ms", "123456789.01µs"},
			D(1234567890123):   {"20m34s", "1234.57s", "1234567.89ms", "1234567890.12µs"},
			D(12345678901234):  {"3h25m45s", "12345.68s", "12345678.90ms", "12345678901.23µs"},
			D(123456789012345): {"34h17m36s", "123456.79s", "123456789.01ms", "123456789012.35µs"},
		},
		{Type: Rate, Contains: Default}: {
			0.0:       {"0.00%", "0.00%", "0.00%", "0.00%"},
			0.01:      {"1.00%", "1.00%", "1.00%", "1.00%"},
			0.02:      {"2.00%", "2.00%", "2.00%", "2.00%"},
			0.022:     {"2.19%", "2.19%", "2.19%", "2.19%"}, // caused by float truncation
			0.0222:    {"2.22%", "2.22%", "2.22%", "2.22%"},
			0.02222:   {"2.22%", "2.22%", "2.22%", "2.22%"},
			0.022222:  {"2.22%", "2.22%", "2.22%", "2.22%"},
			1.0 / 3.0: {"33.33%", "33.33%", "33.33%", "33.33%"},
			0.5:       {"50.00%", "50.00%", "50.00%", "50.00%"},
			0.55:      {"55.00%", "55.00%", "55.00%", "55.00%"},
			0.555:     {"55.50%", "55.50%", "55.50%", "55.50%"},
			0.5555:    {"55.55%", "55.55%", "55.55%", "55.55%"},
			0.55555:   {"55.55%", "55.55%", "55.55%", "55.55%"},
			0.75:      {"75.00%", "75.00%", "75.00%", "75.00%"},
			0.999995:  {"99.99%", "99.99%", "99.99%", "99.99%"},
			1.0:       {"100.00%", "100.00%", "100.00%", "100.00%"},
			1.5:       {"150.00%", "150.00%", "150.00%", "150.00%"},
		},
	}

	for m, values := range data {
		m, values := m, values
		t.Run(fmt.Sprintf("type=%s,contains=%s", m.Type.String(), m.Contains.String()), func(t *testing.T) {
			t.Parallel()
			for v, s := range values {
				t.Run(fmt.Sprintf("v=%f", v), func(t *testing.T) {
					t.Run("no fixed unit", func(t *testing.T) {
						assert.Equal(t, s[0], m.HumanizeValue(v, ""))
					})
					t.Run("unit fixed in s", func(t *testing.T) {
						assert.Equal(t, s[1], m.HumanizeValue(v, "s"))
					})
					t.Run("unit fixed in ms", func(t *testing.T) {
						assert.Equal(t, s[2], m.HumanizeValue(v, "ms"))
					})
					t.Run("unit fixed in µs", func(t *testing.T) {
						assert.Equal(t, s[3], m.HumanizeValue(v, "us"))
					})
				})
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
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
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := New("my_metric", data.Type)
			assert.Equal(t, "my_metric", m.Name)
			assert.IsType(t, data.SinkType, m.Sink)
		})
	}
}

func TestNewSubmetric(t *testing.T) {
	t.Parallel()
	testdata := map[string]struct {
		parent string
		tags   map[string]string
	}{
		"my_metric":                 {"my_metric", nil},
		"my_metric{}":               {"my_metric", nil},
		"my_metric{a}":              {"my_metric", map[string]string{"a": ""}},
		"my_metric{a:1}":            {"my_metric", map[string]string{"a": "1"}},
		"my_metric{ a : 1 }":        {"my_metric", map[string]string{"a": "1"}},
		"my_metric{a,b}":            {"my_metric", map[string]string{"a": "", "b": ""}},
		"my_metric{a:1,b:2}":        {"my_metric", map[string]string{"a": "1", "b": "2"}},
		"my_metric{ a : 1, b : 2 }": {"my_metric", map[string]string{"a": "1", "b": "2"}},
	}

	for name, data := range testdata {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parent, sm := NewSubmetric(name)
			assert.Equal(t, data.parent, parent)
			if data.tags != nil {
				assert.EqualValues(t, data.tags, sm.Tags.tags)
			} else {
				assert.Nil(t, sm.Tags)
			}
		})
	}
}

func TestSampleTags(t *testing.T) {
	t.Parallel()

	// Nil pointer to SampleTags
	var nilTags *SampleTags
	assert.True(t, nilTags.IsEqual(nilTags))
	assert.Equal(t, map[string]string{}, nilTags.CloneTags())

	nilJSON, err := json.Marshal(nilTags)
	assert.NoError(t, err)
	assert.Equal(t, "null", string(nilJSON))

	// Empty SampleTags
	emptyTagMap := map[string]string{}
	emptyTags := NewSampleTags(emptyTagMap)
	assert.Nil(t, emptyTags)
	assert.True(t, emptyTags.IsEqual(emptyTags))
	assert.True(t, emptyTags.IsEqual(nilTags))
	assert.Equal(t, emptyTagMap, emptyTags.CloneTags())

	emptyJSON, err := json.Marshal(emptyTags)
	assert.NoError(t, err)
	assert.Equal(t, "null", string(emptyJSON))

	var emptyTagsUnmarshaled *SampleTags
	err = json.Unmarshal(emptyJSON, &emptyTagsUnmarshaled)
	assert.NoError(t, err)
	assert.Nil(t, emptyTagsUnmarshaled)
	assert.True(t, emptyTagsUnmarshaled.IsEqual(emptyTags))
	assert.True(t, emptyTagsUnmarshaled.IsEqual(nilTags))
	assert.Equal(t, emptyTagMap, emptyTagsUnmarshaled.CloneTags())

	// SampleTags with keys and values
	tagMap := map[string]string{"key1": "val1", "key2": "val2"}
	tags := NewSampleTags(tagMap)
	assert.NotNil(t, tags)
	assert.True(t, tags.IsEqual(tags))
	assert.False(t, tags.IsEqual(nilTags))
	assert.False(t, tags.IsEqual(emptyTags))
	assert.False(t, tags.IsEqual(IntoSampleTags(&map[string]string{"key1": "val1", "key2": "val3"})))
	assert.True(t, tags.Contains(IntoSampleTags(&map[string]string{"key1": "val1"})))
	assert.False(t, tags.Contains(IntoSampleTags(&map[string]string{"key3": "val1"})))
	assert.Equal(t, tagMap, tags.CloneTags())

	assert.Nil(t, tags.json) // No cache
	tagsJSON, err := json.Marshal(tags)
	expJSON := `{"key1":"val1","key2":"val2"}`
	assert.NoError(t, err)
	assert.JSONEq(t, expJSON, string(tagsJSON))
	assert.JSONEq(t, expJSON, string(tags.json)) // Populated cache

	var tagsUnmarshaled *SampleTags
	err = json.Unmarshal(tagsJSON, &tagsUnmarshaled)
	assert.NoError(t, err)
	assert.NotNil(t, tagsUnmarshaled)
	assert.True(t, tagsUnmarshaled.IsEqual(tags))
	assert.False(t, tagsUnmarshaled.IsEqual(nilTags))
	assert.Equal(t, tagMap, tagsUnmarshaled.CloneTags())
}

func TestSampleImplementations(t *testing.T) {
	tagMap := map[string]string{"key1": "val1", "key2": "val2"}
	now := time.Now()

	sample := Sample{
		Metric: New("test_metric", Counter),
		Time:   now,
		Tags:   NewSampleTags(tagMap),
		Value:  1.0,
	}
	samples := Samples(sample.GetSamples())
	cSamples := ConnectedSamples{
		Samples: []Sample{sample},
		Time:    now,
		Tags:    NewSampleTags(tagMap),
	}
	exp := []Sample{sample}
	assert.Equal(t, exp, sample.GetSamples())
	assert.Equal(t, exp, samples.GetSamples())
	assert.Equal(t, exp, cSamples.GetSamples())
	assert.Equal(t, now, sample.GetTime())
	assert.Equal(t, now, cSamples.GetTime())
	assert.Equal(t, sample.GetTags(), sample.GetTags())
}

func TestGetResolversForTrendColumnsValidation(t *testing.T) {
	validateTests := []struct {
		stats  []string
		expErr bool
	}{
		{[]string{}, false},
		{[]string{"avg", "min", "med", "max", "p(0)", "p(99)", "p(99.999)", "count"}, false},
		{[]string{"avg", "p(err)"}, true},
		{[]string{"nil", "p(err)"}, true},
		{[]string{"p90"}, true},
		{[]string{"p(90"}, true},
		{[]string{" avg"}, true},
		{[]string{"avg "}, true},
		{[]string{"", "avg "}, true},
		{[]string{"p(-1)"}, true},
		{[]string{"p(101)"}, true},
		{[]string{"p(1)"}, false},
	}

	for _, tc := range validateTests {
		tc := tc
		t.Run(fmt.Sprintf("%v", tc.stats), func(t *testing.T) {
			_, err := GetResolversForTrendColumns(tc.stats)
			if tc.expErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func createTestTrendSink(count int) *TrendSink {
	sink := TrendSink{}

	for i := 0; i < count; i++ {
		sink.Add(Sample{Value: float64(i)})
	}

	return &sink
}

func TestResolversForTrendColumnsCalculation(t *testing.T) {
	customResolversTests := []struct {
		stats      string
		percentile float64
	}{
		{"p(50)", 0.5},
		{"p(99)", 0.99},
		{"p(99.9)", 0.999},
		{"p(99.99)", 0.9999},
		{"p(99.999)", 0.99999},
	}

	sink := createTestTrendSink(100)

	for _, tc := range customResolversTests {
		tc := tc
		t.Run(fmt.Sprintf("%v", tc.stats), func(t *testing.T) {
			res, err := GetResolversForTrendColumns([]string{tc.stats})
			assert.NoError(t, err)
			assert.Len(t, res, 1)
			for k := range res {
				assert.InDelta(t, sink.P(tc.percentile), res[k](sink), 0.000001)
			}
		})
	}
}
