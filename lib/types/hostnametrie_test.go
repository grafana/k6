/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostnameTrieInsert(t *testing.T) {
	hostnames, err := NewHostnameTrie([]string{"foo.bar"})
	assert.NoError(t, err)
	assert.NoError(t, hostnames.insert("test.k6.io"))
	assert.Error(t, hostnames.insert("inval*d.pattern"))
	assert.NoError(t, hostnames.insert("*valid.pattern"))
}

func TestHostnameTrieContains(t *testing.T) {
	trie, err := NewHostnameTrie([]string{"sub.test.k6.io", "test.k6.io", "*valid.pattern", "sub.valid.pattern"})
	require.NoError(t, err)
	cases := map[string]string{
		"K6.Io":                 "",
		"tEsT.k6.Io":            "test.k6.io",
		"TESt.K6.IO":            "test.k6.io",
		"sub.test.k6.io":        "sub.test.k6.io",
		"sub.sub.test.k6.io":    "",
		"blocked.valId.paTtern": "*valid.pattern",
		"valId.paTtern":         "*valid.pattern",
		"sub.valid.pattern":     "sub.valid.pattern", // use the most specific blocker
		"www.sub.valid.pattern": "*valid.pattern",
		"example.test.k6.io":    "",
	}
	for key, value := range cases {
		host, pattern := key, value
		t.Run(host, func(t *testing.T) {
			match, matches := trie.Contains(host)
			if pattern == "" {
				assert.False(t, matches)
				assert.Empty(t, match)
			} else {
				assert.True(t, matches)
				assert.Equal(t, pattern, match)
			}
		})
	}
}
