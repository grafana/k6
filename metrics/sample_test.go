package metrics

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
		Metric: newMetric("test_metric", Counter),
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

func TestGetResolversForTrendColumnsCalculation(t *testing.T) {
	t.Parallel()

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

	for _, tc := range customResolversTests {
		tc := tc
		t.Run(fmt.Sprintf("%v", tc.stats), func(t *testing.T) {
			t.Parallel()
			sink := createTestTrendSink(100)

			res, err := GetResolversForTrendColumns([]string{tc.stats})
			assert.NoError(t, err)
			assert.Len(t, res, 1)
			for k := range res {
				assert.InDelta(t, sink.P(tc.percentile), res[k](sink), 0.000001)
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
