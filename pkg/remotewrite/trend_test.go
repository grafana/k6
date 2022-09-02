package remotewrite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/metrics"
)

// check that ad-hoc optimization doesn't produce wrong values
func TestTrendSinkAdd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		current  trendSink
		s        metrics.Sample
		expected trendSink
	}{
		{
			current: trendSink{},
			s:       metrics.Sample{Value: 2},
			expected: trendSink{
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
			current: trendSink{
				Values: []float64{1, 2, 3, 4, 7, 8}, // expected to be sorted
				Count:  6,
				Min:    1,
				Max:    8,
				Sum:    25,
			},
			s: metrics.Sample{Value: 12.3},
			expected: trendSink{
				Values: []float64{1, 2, 3, 4, 7, 8, 12.3},
				Count:  7,
				Min:    1,
				Max:    12.3,
				Sum:    37.3,
				Avg:    37.3 / 7,
				Med:    4,
			},
		},
	}

	for _, testCase := range testCases {
		testCase.current.Add(testCase.s)
		sink := testCase.current

		k6sink := &metrics.TrendSink{}
		for _, v := range sink.Values {
			k6sink.Add(metrics.Sample{Value: v})
		}
		// the k6 metrics.TrendSink and the modified version in this repo
		// must return the same values
		assert.Equal(t, sink.Format(0), k6sink.Format(0))

		require.Equal(t, testCase.expected.Values, sink.Values)
		assert.Equal(t, testCase.expected.Count, sink.Count)
		assert.Equal(t, testCase.expected.Min, sink.Min)
		assert.Equal(t, testCase.expected.Max, sink.Max)
		assert.Equal(t, testCase.expected.Sum, sink.Sum)
		assert.Equal(t, testCase.expected.Avg, sink.Avg)
		assert.Equal(t, testCase.expected.Med, sink.Med)
	}
}

// TODO: update and recheck the results

// func BenchmarkTrendAdd(b *testing.B) {
// benchF := []func(b *testing.B, start metrics.Metric){
// func(b *testing.B, m metrics.Metric) {
// b.ResetTimer()
// rand.Seed(time.Now().Unix())

//for i := 0; i < b.N; i++ {
//trendAdd(&m, metrics.Sample{Value: rand.Float64() * 1000})
//sink := m.Sink.(*metrics.TrendSink)
//p(sink, 0.90)
//p(sink, 0.95)
//}
//},
//func(b *testing.B, start metrics.Metric) {
//b.ResetTimer()
//rand.Seed(time.Now().Unix())

//for i := 0; i < b.N; i++ {
//start.Sink.Add(metrics.Sample{Value: rand.Float64() * 1000})
//start.Sink.Format(0)
//}
//},
//}

//start := metrics.Metric{
//Type: metrics.Trend,
//Sink: &metrics.TrendSink{},
//}

//b.Run("trendAdd", func(b *testing.B) {
//benchF[0](b, start)
//})
//b.Run("TrendSink.Add", func(b *testing.B) {
//benchF[1](b, start)
//})
//}
