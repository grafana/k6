package metrics

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/mstoykov/atlas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSampleTagsIsEqual(t *testing.T) {
	t.Parallel()

	// Nil pointer to SampleTags
	var nilTags *SampleTags
	require.Nil(t, nilTags)
	assert.True(t, nilTags.IsEqual(nilTags))

	root := newSampleTagsRoot()

	// Empty set
	emptyTags := root
	assert.True(t, emptyTags.IsEqual(root))
	assert.True(t, emptyTags.IsEqual(emptyTags))
	assert.False(t, emptyTags.IsEqual(nilTags))

	// Different roots
	addTagToSampleTags(root, "tag1", "value1")

	root2 := newSampleTagsRoot()
	addTagToSampleTags(root, "tag1", "value1")
	assert.False(t, root.IsEqual(root2))

	// When the other tag set is nil
	assert.False(t, root.IsEqual(nilTags))
}

func TestSampleTagsIsEmpty(t *testing.T) {
	t.Parallel()

	st := newSampleTagsRoot()
	assert.True(t, st.IsEmpty())

	addTagToSampleTags(st, "key1", "val1")
	assert.False(t, st.IsEmpty())
}

func TestSampleTagsGet(t *testing.T) {
	t.Parallel()

	st := newSampleTagsRoot()
	v, ok := st.Get("key1")
	assert.False(t, ok)
	assert.Empty(t, v)

	addTagToSampleTags(st, "key1", "val1")
	v, ok = st.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "val1", v)
}

func TestSampleTagsCloneTags(t *testing.T) {
	t.Parallel()

	st := newSampleTagsRoot()
	addTagToSampleTags(st, "key1", "val1")
	addTagToSampleTags(st, "key2", "val2")
	addTagToSampleTags(st, "key3", "val3")

	expected := map[string]string{
		"key1": "val1",
		"key2": "val2",
		"key3": "val3",
	}
	assert.NotNil(t, expected, st.CloneTags())
}

func TestSampleTagsContains(t *testing.T) {
	t.Parallel()

	st := newSampleTagsRoot()
	addTagToSampleTags(st, "maintag", "mainvalue")

	branch := &SampleTags{tags: st.tags}
	addTagToSampleTags(branch, "tag1", "val1")
	addTagToSampleTags(branch, "tag2", "val2")

	inner := &SampleTags{tags: st.tags}
	addTagToSampleTags(inner, "tag1", "val1")

	outer := &SampleTags{tags: st.tags}
	addTagToSampleTags(outer, "tag3", "val2")

	assert.True(t, st.Contains(st))
	assert.True(t, branch.Contains(st))
	assert.True(t, branch.Contains(inner))
	assert.False(t, branch.Contains(outer))
	assert.False(t, st.Contains(outer))
}

func TestSampleTagsJSON(t *testing.T) {
	t.Parallel()

	var nilTags *SampleTags
	nilJSON, err := json.Marshal(nilTags)
	assert.NoError(t, err)
	assert.Equal(t, "null", string(nilJSON))

	root := newSampleTagsRoot()

	tags := &SampleTags{tags: root.tags}
	addTagToSampleTags(tags, "key1", "val1")
	addTagToSampleTags(tags, "key2", "val2")

	assert.Nil(t, tags.json) // No cache
	tagsJSON, err := json.Marshal(tags)
	require.NoError(t, err)
	expJSON := `{"key1":"val1","key2":"val2"}`
	assert.JSONEq(t, expJSON, string(tagsJSON))
	assert.JSONEq(t, expJSON, string(tags.json)) // Populated cache

	emptyTags := newSampleTagsRoot()
	emptyJSON, err := json.Marshal(emptyTags)
	require.NoError(t, err)
	assert.Equal(t, "null", string(emptyJSON))

	var emptyTagsUnmarshaled *SampleTags
	err = json.Unmarshal(emptyJSON, &emptyTagsUnmarshaled)
	require.NoError(t, err)
	assert.Nil(t, emptyTagsUnmarshaled)
	assert.True(t, emptyTagsUnmarshaled.IsEqual(nilTags))
	assert.Equal(t, map[string]string{}, emptyTagsUnmarshaled.CloneTags())

	tagsUnmarshaled := &SampleTags{tags: root.tags}
	err = json.Unmarshal(tagsJSON, &tagsUnmarshaled)
	require.NoError(t, err)
	assert.NotNil(t, tagsUnmarshaled)
	assert.True(t, tagsUnmarshaled.IsEqual(tags))
	assert.False(t, tagsUnmarshaled.IsEqual(nilTags))

	exp := map[string]string{"key1": "val1", "key2": "val2"}
	assert.Equal(t, exp, tagsUnmarshaled.CloneTags())
}

func TestSampleImplementations(t *testing.T) {
	tagMap := NewTagSet(map[string]string{"key1": "val1", "key2": "val2"})
	sampleTags := tagMap.SampleTags()
	now := time.Now()

	sample := Sample{
		Metric: newMetric("test_metric", Counter),
		Time:   now,
		Tags:   sampleTags,
		Value:  1.0,
	}
	samples := Samples(sample.GetSamples())
	cSamples := ConnectedSamples{
		Samples: []Sample{sample},
		Time:    now,
		Tags:    sampleTags,
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

func newSampleTagsRoot() *SampleTags {
	return &SampleTags{
		tags: atlas.New(),
	}
}

func addTagToSampleTags(st *SampleTags, key, value string) {
	st.tags = st.tags.AddLink(key, value)
}
