package remotewrite

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/metrics"
)

// check that ad-hoc optimization doesn't produce wrong values
func TestTrendAdd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		current  *metrics.Metric
		s        metrics.Sample
		expected metrics.TrendSink
	}{
		{
			current: &metrics.Metric{
				Sink: &metrics.TrendSink{},
			},
			s: metrics.Sample{Value: 2},
			expected: metrics.TrendSink{
				Values: []float64{2},
				Count:  1,
				Min:    2,
				Max:    2,
				Sum:    2,
				Avg:    2,
				Med:    2,
			},
		},
		{
			current: &metrics.Metric{
				Sink: &metrics.TrendSink{
					Values: []float64{8, 3, 1, 7, 4, 2},
					Count:  6,
					Min:    1,
					Max:    8,
					Sum:    25,
				},
			},
			s: metrics.Sample{Value: 12.3},
			expected: metrics.TrendSink{
				Values: []float64{8, 3, 1, 7, 4, 2, 12.3},
				Count:  7,
				Min:    1,
				Max:    12.3,
				Sum:    37.3,
				Avg:    37.3 / 7,
				Med:    7,
			},
		},
	}

	for _, testCase := range testCases {
		// trendAdd should result in the same values as Sink.Add

		trendAdd(testCase.current, testCase.s)
		sink := testCase.current.Sink.(*metrics.TrendSink)

		assert.Equal(t, testCase.expected.Count, sink.Count)
		assert.Equal(t, testCase.expected.Min, sink.Min)
		assert.Equal(t, testCase.expected.Max, sink.Max)
		assert.Equal(t, testCase.expected.Sum, sink.Sum)
		assert.Equal(t, testCase.expected.Avg, sink.Avg)
		assert.Equal(t, testCase.expected.Med, sink.Med)
		assert.Equal(t, testCase.expected.Values, sink.Values)
	}
}

func BenchmarkTrendAdd(b *testing.B) {
	benchF := []func(b *testing.B, start metrics.Metric){
		func(b *testing.B, m metrics.Metric) {
			b.ResetTimer()
			rand.Seed(time.Now().Unix())

			for i := 0; i < b.N; i++ {
				trendAdd(&m, metrics.Sample{Value: rand.Float64() * 1000})
				sink := m.Sink.(*metrics.TrendSink)
				p(sink, 0.90)
				p(sink, 0.95)
			}
		},
		func(b *testing.B, start metrics.Metric) {
			b.ResetTimer()
			rand.Seed(time.Now().Unix())

			for i := 0; i < b.N; i++ {
				start.Sink.Add(metrics.Sample{Value: rand.Float64() * 1000})
				start.Sink.Format(0)
			}
		},
	}

	start := metrics.Metric{
		Type: metrics.Trend,
		Sink: &metrics.TrendSink{},
	}

	b.Run("trendAdd", func(b *testing.B) {
		benchF[0](b, start)
	})
	b.Run("TrendSink.Add", func(b *testing.B) {
		benchF[1](b, start)
	})
}
