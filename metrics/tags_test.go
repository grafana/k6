package metrics

import (
	"encoding/json"
	"testing"

	"github.com/mstoykov/atlas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagSetBranchOut(t *testing.T) {
	t.Parallel()

	tm := newTagSet(nil)
	tm.AddTag("key1", "val1")

	tm2 := tm.BranchOut()
	tm2.AddTag("key2", "val2")

	tm.AddTag("key3", "val3")

	assert.Equal(t, map[string]string{"key1": "val1", "key3": "val3"}, tm.Map())
	assert.Equal(t, map[string]string{
		"key1": "val1",
		"key2": "val2",
	}, tm2.Map())
}

func TestTagSetFromSampleTags(t *testing.T) {
	t.Parallel()

	rootNode := atlas.New()
	rootNode = rootNode.AddLink("key1", "val1")
	st := &SampleTags{tags: rootNode}

	tm := TagSetFromSampleTags(st)
	tm.AddTag("key2", "val2")

	assert.Equal(t, map[string]string{"key1": "val1"}, st.CloneTags())
	assert.Equal(t, map[string]string{
		"key1": "val1",
		"key2": "val2",
	}, tm.Map())
}

func TestEnabledTagsMarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tagset   EnabledTags
		expected string
	}{
		{tagset: EnabledTags{"ip": true, "proto": true, "group": true, "custom": true}, expected: `["custom","group","ip","proto"]`},
		{tagset: EnabledTags{}, expected: `[]`},
	}

	for _, tc := range tests {
		ts := &tc.tagset
		got, err := json.Marshal(ts)
		require.Nil(t, err)
		require.Equal(t, tc.expected, string(got))
	}
}

func TestEnabledTagsUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tags []byte
		sets EnabledTags
	}{
		{[]byte(`[]`), EnabledTags{}},
		{[]byte(`["ip","custom", "proto"]`), EnabledTags{"ip": true, "proto": true, "custom": true}},
	}

	for _, tc := range tests {
		ts := new(EnabledTags)
		require.Nil(t, json.Unmarshal(tc.tags, ts))
		for tag := range tc.sets {
			assert.True(t, (*ts)[tag])
		}
	}
}

func TestEnabledTagsTextUnmarshal(t *testing.T) {
	t.Parallel()

	testMatrix := map[string]EnabledTags{
		"":                           make(EnabledTags),
		"ip":                         {"ip": true},
		"ip,proto":                   {"ip": true, "proto": true},
		"   ip  ,  proto  ":          {"ip": true, "proto": true},
		"   ip  ,   ,  proto  ":      {"ip": true, "proto": true},
		"   ip  ,,  proto  ,,":       {"ip": true, "proto": true},
		"   ip  ,custom,  proto  ,,": {"ip": true, "custom": true, "proto": true},
	}

	for input, expected := range testMatrix {
		set := new(EnabledTags)
		err := set.UnmarshalText([]byte(input))
		require.NoError(t, err)
		require.Equal(t, expected, *set)
	}
}

func newTagSet(m map[string]string) *TagSet {
	node := atlas.New()
	for k, v := range m {
		node = node.AddLink(k, v)
	}

	return &TagSet{
		tags: node,
	}
}
