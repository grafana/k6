package loki

import (
	"context"
	"testing"

	gofakeit "github.com/brianvoe/gofakeit/v6"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func BenchmarkNewBatch(b *testing.B) {
	samples := make(chan metrics.SampleContainer)
	state := &lib.State{
		Samples: samples,
		VUID:    15,
	}
	ctx, cancel := context.WithCancel(context.Background())

	vu := &modulestest.VU{
		CtxField:   ctx,
		StateField: state,
	}
	defer cancel()
	defer close(samples)
	go func() { // this is so that we read the send samples
		for range samples {
		}
	}()
	faker := gofakeit.New(12345)
	cardinalities := map[string]int{
		"app":       5,
		"namespace": 10,
		"pod":       100,
	}
	streams, minBatchSize, maxBatchSize := 5, 500, 1000
	labels := newLabelPool(faker, cardinalities)

	c := Client{
		vu:     vu,
		labels: transformLabelPool(labels),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = c.newBatch(streams, minBatchSize, maxBatchSize)
	}
}

func BenchmarkEncode(b *testing.B) {
	samples := make(chan metrics.SampleContainer)
	state := &lib.State{
		Samples: samples,
		VUID:    15,
	}
	ctx, cancel := context.WithCancel(context.Background())

	vu := &modulestest.VU{
		CtxField:   ctx,
		StateField: state,
	}

	defer cancel()
	defer close(samples)
	go func() { // this is so that we read the send samples
		for range samples {
		}
	}()
	faker := gofakeit.New(12345)
	cardinalities := map[string]int{
		"app":       5,
		"namespace": 10,
		"pod":       100,
	}
	streams, minBatchSize, maxBatchSize := 5, 500, 1000
	labels := newLabelPool(faker, cardinalities)

	c := Client{
		vu:     vu,
		labels: transformLabelPool(labels),
	}
	batch := c.newBatch(streams, minBatchSize, maxBatchSize)

	b.Run("encode protobuf", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = batch.encodeSnappy()
		}
	})

	b.Run("encode json", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = batch.encodeJSON()
		}
	})
}
