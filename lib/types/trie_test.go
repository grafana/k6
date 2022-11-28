package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrieInsert(t *testing.T) {
	t.Parallel()

	root := trieNode{}

	const val = "k6.io"
	root.insert(val)

	ptr := &root
	for i, rs := len(val)-1, []rune(val); i >= 0; i-- {
		next, ok := ptr.children[rs[i]]
		require.True(t, ok)
		ptr = next
	}

	require.True(t, ptr.isLeaf)
}

func TestTrieContains(t *testing.T) {
	t.Parallel()

	root := trieNode{}
	root.insert("k6.io")
	root.insert("specific.k6.io")
	root.insert("*.k6.io")

	tcs := []struct {
		query, expVal string
		found         bool
	}{
		// Trie functionality
		{query: "k6.io", expVal: "k6.io", found: true},
		{query: "io", expVal: "", found: false},
		{query: "no.k6.no.io", expVal: "", found: false},
		{query: "specific.k6.io", expVal: "specific.k6.io", found: true},
		{query: "", expVal: "", found: false},
		{query: "long.long.long.long.long.long.long.long.no.match", expVal: "", found: false},
		{query: "pre.matching.long.long.long.long.test.k6.noio", expVal: "", found: false},

		// Wildcard
		{query: "foo.k6.io", expVal: "*.k6.io", found: true},
		{query: "specific.k6.io", expVal: "specific.k6.io", found: true},
		{query: "not.specific.k6.io", expVal: "*.k6.io", found: true},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.query, func(t *testing.T) {
			t.Parallel()

			val, ok := root.contains(tc.query)

			require.Equal(t, tc.found, ok)
			require.Equal(t, tc.expVal, val)
		})
	}
}

func TestReverseString(t *testing.T) {
	t.Parallel()

	tcs := []struct{ str, rev string }{
		{str: "even", rev: "neve"},
		{str: "odd", rev: "ddo"},
		{str: "", rev: ""},
	}

	for _, tc := range tcs {
		tc := tc

		t.Run(tc.str, func(t *testing.T) {
			t.Parallel()
			val := reverseString(tc.str)

			require.Equal(t, tc.rev, val)
		})
	}
}

func BenchmarkTrieInsert(b *testing.B) {
	arr := []string{
		"k6.io", "*.sub.k6.io", "specific.sub.k6.io",
		"grafana.com", "*.sub.sub.grafana.com", "test.sub.sub.grafana.com",
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root := trieNode{}
		for _, v := range arr {
			root.insert(v)
		}
	}
}

func BenchmarkTrieContains(b *testing.B) {
	root := trieNode{}
	arr := []string{
		"k6.io", "*.sub.k6.io", "specific.sub.k6.io",
		"grafana.com", "*.sub.sub.grafana.com", "test.sub.sub.grafana.com",
	}

	for _, v := range arr {
		root.insert(v)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, v := range arr {
			root.contains(v)
		}
	}
}
