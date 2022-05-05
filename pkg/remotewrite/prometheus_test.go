package remotewrite

import (
	"testing"
	"time"

	"math/rand"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/metrics"
)

// check that ad-hoc optimization doesn't produce wrong values
func TestTrendAdd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		current, s metrics.Sample
	}{
		{
			current: metrics.Sample{Metric: &metrics.Metric{
				Sink: &metrics.TrendSink{},
			}},
			s: metrics.Sample{Value: 2},
		},
		{
			current: metrics.Sample{Metric: &metrics.Metric{
				Sink: &metrics.TrendSink{
					Values: []float64{8, 3, 1, 7, 4, 2},
					Count:  6,
					Min:    1, Max: 8,
					Sum: 25, Avg: (8 + 3 + 1 + 7 + 4 + 2) / 6,
					Med: (3 + 4) / 2,
				},
			}},
			s: metrics.Sample{Value: 12.3},
		},
	}

	for _, testCase := range testCases {
		// trendAdd should result in the same values as Sink.Add

		s := trendAdd(testCase.current, testCase.s)
		sink := s.Metric.Sink.(*metrics.TrendSink)

		testCase.current.Metric.Sink.Add(testCase.s)
		expected := testCase.current.Metric.Sink.(*metrics.TrendSink)

		assert.Equal(t, expected.Count, sink.Count)
		assert.Equal(t, expected.Min, sink.Min)
		assert.Equal(t, expected.Max, sink.Max)
		assert.Equal(t, expected.Sum, sink.Sum)
		assert.Equal(t, expected.Avg, sink.Avg)
		assert.EqualValues(t, expected.Values, sink.Values)
	}
}

func BenchmarkTrendAdd(b *testing.B) {
	benchF := []func(b *testing.B, start metrics.Sample){
		func(b *testing.B, s metrics.Sample) {
			b.ResetTimer()
			rand.Seed(time.Now().Unix())

			for i := 0; i < b.N; i++ {
				s = trendAdd(s, metrics.Sample{Value: rand.Float64() * 1000})
				sink := s.Metric.Sink.(*metrics.TrendSink)
				p(sink, 0.90)
				p(sink, 0.95)
			}
		},
		func(b *testing.B, start metrics.Sample) {
			b.ResetTimer()
			rand.Seed(time.Now().Unix())

			for i := 0; i < b.N; i++ {
				start.Metric.Sink.Add(metrics.Sample{Value: rand.Float64() * 1000})
				start.Metric.Sink.Format(0)
			}
		},
	}

	s := metrics.Sample{
		Metric: &metrics.Metric{
			Type: metrics.Trend,
			Sink: &metrics.TrendSink{},
		},
	}

	b.Run("trendAdd", func(b *testing.B) {
		benchF[0](b, s)
	})
	b.Run("TrendSink.Add", func(b *testing.B) {
		benchF[1](b, s)
	})
}
