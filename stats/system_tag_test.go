package stats

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemTagSetMarshalJSON(t *testing.T) {
	var tests = []struct {
		tagset   SystemTagSet
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

func TestSystemTagSet_UnmarshalJSON(t *testing.T) {
	var tests = []struct {
		tags []byte
		sets []SystemTagSet
	}{
		{[]byte(`[]`), []SystemTagSet{}},
		{[]byte(`["ip", "proto"]`), []SystemTagSet{IP, Proto}},
	}

	for _, tc := range tests {
		ts := new(SystemTagSet)
		require.Nil(t, json.Unmarshal(tc.tags, ts))
		for _, tag := range tc.sets {
			assert.True(t, ts.Has(tag))
		}
	}

}

func TestSystemTagSetTextUnmarshal(t *testing.T) {
	var testMatrix = map[string]SystemTagSet{
		"":                      0,
		"ip":                    IP,
		"ip,proto":              IP | Proto,
		"   ip  ,  proto  ":     IP | Proto,
		"   ip  ,   ,  proto  ": IP | Proto,
		"   ip  ,,  proto  ,,":  IP | Proto,
	}

	for input, expected := range testMatrix {
		var set = new(SystemTagSet)
		err := set.UnmarshalText([]byte(input))
		require.NoError(t, err)
		require.Equal(t, expected, *set)
	}
}
