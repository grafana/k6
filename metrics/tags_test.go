package metrics

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
