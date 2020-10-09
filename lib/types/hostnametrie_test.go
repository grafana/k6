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
