package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sink interface{}
		mt   MetricType
	}{
		{mt: Counter, sink: &CounterSink{}},
		{mt: Gauge, sink: &GaugeSink{}},
		{mt: Rate, sink: &RateSink{}},
		{mt: Trend, sink: NewTrendSink()},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.sink, NewSink(tc.mt))
	}
}

func TestNewSinkInvalidMetricType(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { NewSink(MetricType(6)) })
}

func TestCounterSink(t *testing.T) {
	t.Parallel()
	samples10 := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 100.0}
	now := time.Now()

	t.Run("add", func(t *testing.T) {
		t.Parallel()
		t.Run("one value", func(t *testing.T) {
			t.Parallel()
			sink := CounterSink{}
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 1.0, Time: now})
			assert.Equal(t, 1.0, sink.Value)
			assert.Equal(t, now, sink.First)
		})
		t.Run("values", func(t *testing.T) {
			t.Parallel()
			sink := CounterSink{}
			for _, s := range samples10 {
				sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s, Time: now})
			}
			assert.Equal(t, 145.0, sink.Value)
			assert.Equal(t, now, sink.First)
		})
	})
	t.Run("format", func(t *testing.T) {
		t.Parallel()
		sink := CounterSink{}
		for _, s := range samples10 {
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s, Time: now})
		}
		assert.Equal(t, map[string]float64{"count": 145, "rate": 145.0}, sink.Format(1*time.Second))
	})
}

func TestGaugeSink(t *testing.T) {
	t.Parallel()
	samples6 := []float64{1.0, 2.0, 3.0, 4.0, 10.0, 5.0}

	t.Run("add", func(t *testing.T) {
		t.Parallel()
		t.Run("one value", func(t *testing.T) {
			t.Parallel()
			sink := GaugeSink{}
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 1.0})
			assert.Equal(t, 1.0, sink.Value)
			assert.Equal(t, 1.0, sink.Min)
			assert.Equal(t, true, sink.minSet)
			assert.Equal(t, 1.0, sink.Max)
		})
		t.Run("values", func(t *testing.T) {
			t.Parallel()
			sink := GaugeSink{}
			for _, s := range samples6 {
				sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
			}
			assert.Equal(t, 5.0, sink.Value)
			assert.Equal(t, 1.0, sink.Min)
			assert.Equal(t, true, sink.minSet)
			assert.Equal(t, 10.0, sink.Max)
		})
	})
	t.Run("format", func(t *testing.T) {
		t.Parallel()
		sink := GaugeSink{}
		for _, s := range samples6 {
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
		}
		assert.Equal(t, map[string]float64{"value": 5.0}, sink.Format(0))
	})
}

func TestTrendSink(t *testing.T) {
	t.Parallel()

	unsortedSamples10 := []float64{
		0.120, // 120ms
		0.035, // 35ms
		0.015, // 15ms
		0.190, // 190ms
		0.280, // 280ms
		0.001, // 1ms
		0.070, // 70ms
		0.499, // 499ms
		1.200, // 1.2s
		2.500, // 2.5s
	}

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		sink := NewTrendSink()
		assert.True(t, sink.IsEmpty())
		assert.Equal(t, float64(0), sink.Min())
		assert.Equal(t, float64(0), sink.Max())
		assert.Equal(t, float64(0), sink.Avg())
		assert.Equal(t, uint64(0), sink.Count())
		assert.Equal(t, float64(0), sink.P(0))
		assert.Equal(t, float64(0), sink.P(0.99))
	})

	t.Run("add", func(t *testing.T) {
		t.Run("one value", func(t *testing.T) {
			t.Parallel()

			sink := NewTrendSink()
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 0.050}) // 50ms
			assert.Equal(t, uint64(1), sink.Count())
			assert.Equal(t, 0.050, sink.Min())
			assert.Equal(t, 0.050, sink.Max())
			assert.Equal(t, 0.050, sink.Avg())
			assert.Equal(t, 0.050, sink.Total())
		})
		t.Run("values", func(t *testing.T) {
			t.Parallel()

			sink := NewTrendSink()
			for _, s := range unsortedSamples10 {
				sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
			}
			assert.Equal(t, uint64(len(unsortedSamples10)), sink.Count())
			assert.Equal(t, 0.001, sink.Min())
			assert.Equal(t, 2.5, sink.Max())
			assert.Equal(t, 0.491, sink.Avg())
			assert.Equal(t, 4.91, sink.Total())
		})
	})

	tolerance := 0.5
	t.Run("percentile", func(t *testing.T) {
		t.Run("no values", func(t *testing.T) {
			t.Parallel()

			sink := NewTrendSink()
			for i := 1; i <= 100; i++ {
				assert.Equal(t, 0.0, sink.P(float64(i)/100.0))
			}
		})
		t.Run("one value", func(t *testing.T) {
			t.Parallel()

			sink := NewTrendSink()
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 10.0})
			for i := 1; i <= 100; i++ {
				assert.Equal(t, 10.0, sink.P(float64(i)/100.0))
			}
		})
		t.Run("two values", func(t *testing.T) {
			t.Parallel()

			sink := NewTrendSink()
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 5.0})
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 10.0})
			assert.Equal(t, 5.0, sink.P(0.0))
			assert.Equal(t, 7.5, sink.P(0.5))
			assert.Equal(t, 5+(10-5)*0.95, sink.P(0.95))
			assert.Equal(t, 5+(10-5)*0.99, sink.P(0.99))
			assert.Equal(t, 10.0, sink.P(1.0))
		})
		t.Run("more than 2", func(t *testing.T) {
			t.Parallel()

			sink := NewHdrHistogramSink()
			for _, s := range unsortedSamples10 {
				sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
			}
			assert.InDelta(t, 0.001, sink.P(0.0), tolerance)
			assert.InDelta(t, 0.155, sink.P(0.5), tolerance)
			assert.InDelta(t, 1.915, sink.P(0.95), tolerance)
			assert.InDelta(t, 2.383, sink.P(0.99), tolerance)
			assert.InDelta(t, 2.5, sink.P(1.0), tolerance)
		})
	})
	t.Run("format", func(t *testing.T) {
		t.Parallel()

		sink := NewTrendSink()
		for _, s := range unsortedSamples10 {
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
		}
		expected := map[string]float64{
			"min":   0.0,
			"max":   100.0,
			"avg":   54.0,
			"med":   55.0,
			"p(90)": 91.0,
			"p(95)": 95.5,
		}
		result := sink.Format(0)
		require.Equal(t, len(expected), len(result))
		for k, expV := range expected {
			assert.Contains(t, result, k)
			assert.InDelta(t, expV, result[k], tolerance)
		}
	})
}

func TestRateSink(t *testing.T) {
	t.Parallel()
	samples6 := []float64{1.0, 0.0, 1.0, 0.0, 0.0, 1.0}

	t.Run("add", func(t *testing.T) {
		t.Parallel()
		t.Run("one true", func(t *testing.T) {
			t.Parallel()
			sink := RateSink{}
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 1.0})
			assert.Equal(t, int64(1), sink.Total)
			assert.Equal(t, int64(1), sink.Trues)
		})
		t.Run("one false", func(t *testing.T) {
			t.Parallel()
			sink := RateSink{}
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 0.0})
			assert.Equal(t, int64(1), sink.Total)
			assert.Equal(t, int64(0), sink.Trues)
		})
		t.Run("values", func(t *testing.T) {
			t.Parallel()
			sink := RateSink{}
			for _, s := range samples6 {
				sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
			}
			assert.Equal(t, int64(6), sink.Total)
			assert.Equal(t, int64(3), sink.Trues)
		})
	})
	t.Run("format", func(t *testing.T) {
		t.Parallel()
		sink := RateSink{}
		for _, s := range samples6 {
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
		}
		assert.Equal(t, map[string]float64{"rate": 0.5}, sink.Format(0))
	})
}
