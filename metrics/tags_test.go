package metrics

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagsWith(t *testing.T) {
	t.Parallel()

	tm := NewRegistry().RootTagSet().With("key1", "val1")
	tm2 := tm.With("key2", "val2")
	tm = tm.With("key3", "val3")

	assert.Equal(t, map[string]string{"key1": "val1", "key3": "val3"}, tm.Map())
	assert.Equal(t, map[string]string{
		"key1": "val1",
		"key2": "val2",
	}, tm2.Map())

	assert.Equal(t, map[string]string{"key2": "val2"}, tm2.Without("key1").Map())
}

func TestRootTagSet(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	root := r.RootTagSet()
	require.NotNil(t, root)
	assert.True(t, root == r.RootTagSet())
	assert.True(t, root == root.Without("foo"))
	assert.True(t, root.IsEmpty())
	assert.Equal(t, map[string]string{}, root.Map())

	val, ok := root.Get("foo")
	assert.False(t, ok)
	assert.Empty(t, val)

	rJSON, err := root.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, "{}", string(rJSON))
}

func TestTagSets(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	root := r.RootTagSet()

	tags := root.With("tag1", "value1")
	assert.True(t, root != tags)
	assert.True(t, tags == root.With("tag1", "value1"))
	assert.False(t, tags.IsEmpty())
	assert.Equal(t, map[string]string{"tag1": "value1"}, tags.Map())

	tags2 := tags.
		With("tag2", "value2").
		WithTagsFromMap(map[string]string{"tag1": "foo", "tag3": "value3"}).
		Without("tag3")

	assert.Equal(t, map[string]string{"tag1": "foo", "tag2": "value2"}, tags2.Map())

	val, ok := tags2.Get("foo")
	assert.False(t, ok)
	assert.Empty(t, val)

	val, ok = tags2.Get("tag1")
	assert.True(t, ok)
	assert.Equal(t, val, "foo")

	assert.True(t, tags2 == tags2.Without("foo"))

	rJSON, err := tags2.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `{"tag1":"foo","tag2":"value2"}`, string(rJSON))

	assert.True(t, tags2.Contains(root))
	assert.False(t, tags2.Contains(tags))

	tags3 := tags.With("tag1", "foo")
	assert.True(t, tags2.Contains(tags3))
	tags4 := tags3.With("tag3", "value3")
	assert.False(t, tags2.Contains(tags4))
	assert.False(t, tags4.Contains(tags2))
	assert.True(t, tags4.Contains(tags3))
	assert.True(t, tags4.Contains(tags4))
}

func TestTagSetContains(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	root := r.RootTagSet()

	st := root.With("maintag", "mainvalue")

	branch := st.With("tag1", "val1").With("tag2", "val2")
	inner := st.With("tag1", "val1")
	outer := st.With("tag3", "val2")

	assert.True(t, st.Contains(st))
	assert.True(t, branch.Contains(st))
	assert.True(t, branch.Contains(inner))
	assert.False(t, branch.Contains(outer))
	assert.False(t, st.Contains(outer))
}

func TestTagsAndMetaSetTag(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	tm := TagsAndMeta{Tags: r.RootTagSet()}
	tm.SetTag("k1", "v1")
	_, ok := tm.Tags.Get("k1")
	assert.True(t, ok)
}

func TestTagsAndMetaDeleteTag(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	tm := TagsAndMeta{Tags: r.RootTagSet()}
	tm.Tags = tm.Tags.With("k1", "v1")
	_, ok := tm.Tags.Get("k1")
	assert.True(t, ok)

	tm.DeleteTag("k1")
	_, ok = tm.Tags.Get("k1")
	assert.False(t, ok)
}

func TestTagsAndMetaSetMetadata(t *testing.T) {
	t.Parallel()

	t.Run("WhenNil", func(t *testing.T) {
		t.Parallel()
		tm := TagsAndMeta{}
		tm.SetMetadata("k1", "v1")
		_, ok := tm.Metadata["k1"]
		assert.True(t, ok)
	})

	t.Run("WhenNotNil", func(t *testing.T) {
		t.Parallel()
		tm := TagsAndMeta{Metadata: make(map[string]string)}
		tm.SetMetadata("k2", "v2")
		_, ok := tm.Metadata["k2"]
		assert.True(t, ok)
	})
}

func TestTagsAndMetaDeleteMetadata(t *testing.T) {
	t.Parallel()

	tm := TagsAndMeta{Metadata: make(map[string]string)}
	tm.Metadata["k1"] = "v1"
	_, ok := tm.Metadata["k1"]
	assert.True(t, ok)

	tm.DeleteMetadata("k1")
	_, ok = tm.Metadata["k1"]
	assert.False(t, ok)
}

func TestTagsAndMetaSetSystemTagOrMetaIfEnabled(t *testing.T) {
	t.Parallel()
	tm := TagsAndMeta{}

	tm.SetSystemTagOrMetaIfEnabled(&DefaultSystemTagSet, TagIter, "10")
	_, ok := tm.Metadata["iter"]
	assert.False(t, ok)

	tm.SetSystemTagOrMetaIfEnabled(&NonIndexableSystemTags, TagIter, "10")
	_, ok = tm.Metadata["iter"]
	assert.True(t, ok)
}

func TestTagsAndMetaSetSystemTagOrMeta(t *testing.T) {
	t.Parallel()

	t.Run("Tag", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()
		tm := TagsAndMeta{Tags: r.RootTagSet()}
		tm.SetSystemTagOrMeta(TagIter, "10")

		_, ok := tm.Metadata["iter"]
		assert.True(t, ok)
		_, ok = tm.Tags.Get("iter")
		assert.False(t, ok)
	})

	t.Run("Metadata", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()
		tm := TagsAndMeta{Tags: r.RootTagSet()}
		tm.SetSystemTagOrMeta(TagName, "hello-request")

		_, ok := tm.Tags.Get("name")
		assert.True(t, ok)
		_, ok = tm.Metadata["name"]
		assert.False(t, ok)
	})
}

func TestTagsAndMetaClone(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	tags := r.RootTagSet().With("k1", "v1")
	meta := map[string]string{"k2": "v2"}
	tm := TagsAndMeta{Tags: tags, Metadata: meta}

	tm2 := tm.Clone()
	require.NotNil(t, tm2.Tags)
	require.NotNil(t, tm2.Metadata)
	assert.Equal(t, tm, tm2)
	assert.False(t, &tm.Metadata == &tm2.Metadata)
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
