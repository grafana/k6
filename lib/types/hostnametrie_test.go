package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostnameTrieInsert(t *testing.T) {
	t.Parallel()

	hostnames, err := NewHostnameTrie([]string{"foo.bar"})
	assert.NoError(t, err)

	assert.NoError(t, hostnames.insert("test.k6.io"))
	assert.Error(t, hostnames.insert("inval*d.pattern"))
	assert.NoError(t, hostnames.insert("*valid.pattern"))
}

func TestHostnameTrieContains(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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

func TestNullHostnameTrieSource(t *testing.T) {
	t.Parallel()

	trie, err := NewNullHostnameTrie([]string{"sub.test.k6.io", "test.k6.io", "*valid.pattern", "sub.valid.pattern"})

	require.NoError(t, err)

	assert.Equal(t, []string{"sub.test.k6.io", "test.k6.io", "*valid.pattern", "sub.valid.pattern"}, trie.Source())
}
