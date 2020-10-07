/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemTagSetMarshalJSON(t *testing.T) {
	tests := []struct {
		tagset   SystemTagSet
		expected string
	}{
		{TagIP, `["ip"]`},
		{TagIP | TagProto | TagGroup, `["group","ip","proto"]`},
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
	tests := []struct {
		tags []byte
		sets []SystemTagSet
	}{
		{[]byte(`[]`), []SystemTagSet{}},
		{[]byte(`["ip", "proto"]`), []SystemTagSet{TagIP, TagProto}},
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
	testMatrix := map[string]SystemTagSet{
		"":                      0,
		"ip":                    TagIP,
		"ip,proto":              TagIP | TagProto,
		"   ip  ,  proto  ":     TagIP | TagProto,
		"   ip  ,   ,  proto  ": TagIP | TagProto,
		"   ip  ,,  proto  ,,":  TagIP | TagProto,
	}

	for input, expected := range testMatrix {
		set := new(SystemTagSet)
		err := set.UnmarshalText([]byte(input))
		require.NoError(t, err)
		require.Equal(t, expected, *set)
	}
}

func TestTagSetMarshalJSON(t *testing.T) {
	tests := []struct {
		tagset   TagSet
		expected string
	}{
		{tagset: TagSet{"ip": true, "proto": true, "group": true, "custom": true}, expected: `["custom","group","ip","proto"]`},
		{tagset: TagSet{}, expected: `[]`},
	}

	for _, tc := range tests {
		ts := &tc.tagset
		got, err := json.Marshal(ts)
		require.Nil(t, err)
		require.Equal(t, tc.expected, string(got))
	}
}

func TestTagSet_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		tags []byte
		sets TagSet
	}{
		{[]byte(`[]`), TagSet{}},
		{[]byte(`["ip","custom", "proto"]`), TagSet{"ip": true, "proto": true, "custom": true}},
	}

	for _, tc := range tests {
		ts := new(TagSet)
		require.Nil(t, json.Unmarshal(tc.tags, ts))
		for tag := range tc.sets {
			assert.True(t, (*ts)[tag])
		}
	}
}

func TestTagSetTextUnmarshal(t *testing.T) {
	testMatrix := map[string]TagSet{
		"":                           make(TagSet),
		"ip":                         {"ip": true},
		"ip,proto":                   {"ip": true, "proto": true},
		"   ip  ,  proto  ":          {"ip": true, "proto": true},
		"   ip  ,   ,  proto  ":      {"ip": true, "proto": true},
		"   ip  ,,  proto  ,,":       {"ip": true, "proto": true},
		"   ip  ,custom,  proto  ,,": {"ip": true, "custom": true, "proto": true},
	}

	for input, expected := range testMatrix {
		set := new(TagSet)
		err := set.UnmarshalText([]byte(input))
		require.NoError(t, err)
		require.Equal(t, expected, *set)
	}
}
