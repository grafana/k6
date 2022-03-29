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
	"github.com/stretchr/testify/require"
)

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

func TestAddSubmetric(t *testing.T) {
	t.Parallel()
	testdata := map[string]struct {
		err  bool
		tags map[string]string
	}{
		"":                        {true, nil},
		"  ":                      {true, nil},
		"a":                       {false, map[string]string{"a": ""}},
		"a:1":                     {false, map[string]string{"a": "1"}},
		" a : 1 ":                 {false, map[string]string{"a": "1"}},
		"a,b":                     {false, map[string]string{"a": "", "b": ""}},
		` a:"",b: ''`:             {false, map[string]string{"a": "", "b": ""}},
		`a:1,b:2`:                 {false, map[string]string{"a": "1", "b": "2"}},
		` a : 1, b : 2 `:          {false, map[string]string{"a": "1", "b": "2"}},
		`a : '1' , b : "2"`:       {false, map[string]string{"a": "1", "b": "2"}},
		`" a" : ' 1' , b : "2 " `: {false, map[string]string{" a": " 1", "b": "2 "}}, //nolint:gocritic
	}

	for name, expected := range testdata {
		name, expected := name, expected
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := New("metric", Trend)
			sm, err := m.AddSubmetric(name)
			if expected.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sm)
			assert.EqualValues(t, expected.tags, sm.Tags.tags)
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
	assert.False(t, tags.Contains(IntoSampleTags(&map[string]string{"nonexistent_key": ""})))
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
