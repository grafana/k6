package tagset

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagSetMarshalJSON(t *testing.T) {
	var tests = []struct {
		tagset   TagSet
		expected string
	}{
		{IP, `["ip"]`},
		{0, `null`},
	}

	for _, tc := range tests {
		ts := &tc.tagset
		got, err := json.Marshal(ts)
		require.Nil(t, err)
		require.Equal(t, tc.expected, string(got))
	}

}

func TestTagSet_UnmarshalJSON(t *testing.T) {
	var tests = []struct {
		tags []byte
		sets []TagSet
	}{
		{[]byte(`[]`), []TagSet{}},
		{[]byte(`["ip", "proto"]`), []TagSet{IP, Proto}},
	}

	for _, tc := range tests {
		ts := new(TagSet)
		require.Nil(t, json.Unmarshal(tc.tags, ts))
		for _, tag := range tc.sets {
			assert.True(t, ts.Has(tag))
		}
	}

}

func TestTagSetTextUnmarshal(t *testing.T) {
	var testMatrix = map[string]TagSet{
		"":                      0,
		"ip":                    IP,
		"ip,proto":              IP | Proto,
		"   ip  ,  proto  ":     IP | Proto,
		"   ip  ,   ,  proto  ": IP | Proto,
		"   ip  ,,  proto  ,,":  IP | Proto,
	}

	for input, expected := range testMatrix {
		var set = new(TagSet)
		err := set.UnmarshalText([]byte(input))
		require.NoError(t, err)
		require.Equal(t, expected, *set)
	}
}
