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
	hostnames := HostnameTrie{}
	assert.NoError(t, hostnames.insert("test.k6.io"))
	assert.Error(t, hostnames.insert("inval*d.pattern"))
	assert.NoError(t, hostnames.insert("*valid.pattern"))
}

func TestHostnameTrieContains(t *testing.T) {
	trie, err := NewHostnameTrie([]string{"test.k6.io", "*valid.pattern"})
	require.NoError(t, err)
	_, matches := trie.Contains("K6.Io")
	assert.False(t, matches)
	match, matches := trie.Contains("tEsT.k6.Io")
	assert.True(t, matches)
	assert.Equal(t, "test.k6.io", match)
	match, matches = trie.Contains("TEST.K6.IO")
	assert.True(t, matches)
	assert.Equal(t, "test.k6.io", match)
	match, matches = trie.Contains("blocked.valId.paTtern")
	assert.True(t, matches)
	assert.Equal(t, "*valid.pattern", match)
	_, matches = trie.Contains("example.test.k6.io")
	assert.False(t, matches)
}
