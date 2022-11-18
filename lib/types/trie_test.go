package types

import "testing"

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
