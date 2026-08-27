package loki

import (
	"math/rand"
	"testing"

	gofakeit "github.com/brianvoe/gofakeit/v6"
	"github.com/grafana/xk6-loki/flog"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func newBenchmarkClient() Client {
	random := rand.New(rand.NewSource(12345)) //nolint:gosec // deterministic benchmark input
	faker := gofakeit.NewCustom(random)
	labels := newLabelPool(faker, map[string]int{
		"app": 5, "namespace": 10, "pod": 100,
	})
	return Client{
		vu:     &extensionapitest.VU{VUIDValue: 15},
		rand:   random,
		faker:  faker,
		flog:   flog.New(random, faker),
		labels: transformLabelPool(labels),
	}
}

func BenchmarkNewBatch(b *testing.B) {
	c := newBenchmarkClient()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = c.newBatch(5, 500, 1000)
	}
}

func BenchmarkEncode(b *testing.B) {
	c := newBenchmarkClient()
	batch := c.newBatch(5, 500, 1000)
	b.Run("encode protobuf", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, _, _ = batch.encodeSnappy()
		}
	})
	b.Run("encode json", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_, _, _ = batch.encodeJSON()
		}
	})
}
