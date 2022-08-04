package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCounterSink(t *testing.T) {
	samples10 := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 100.0}
	now := time.Now()

	t.Run("add", func(t *testing.T) {
		t.Run("one value", func(t *testing.T) {
			sink := CounterSink{}
			sink.Add(Sample{Metric: &Metric{}, Value: 1.0, Time: now})
			assert.Equal(t, 1.0, sink.Value)
			assert.Equal(t, now, sink.First)
		})
		t.Run("values", func(t *testing.T) {
			sink := CounterSink{}
			for _, s := range samples10 {
				sink.Add(Sample{Metric: &Metric{}, Value: s, Time: now})
			}
			assert.Equal(t, 145.0, sink.Value)
			assert.Equal(t, now, sink.First)
		})
	})
	t.Run("calc", func(t *testing.T) {
		sink := CounterSink{}
		sink.Calc()
		assert.Equal(t, 0.0, sink.Value)
		assert.Equal(t, time.Time{}, sink.First)
	})
	t.Run("format", func(t *testing.T) {
		sink := CounterSink{}
		for _, s := range samples10 {
			sink.Add(Sample{Metric: &Metric{}, Value: s, Time: now})
		}
		assert.Equal(t, map[string]float64{"count": 145, "rate": 145.0}, sink.Format(1*time.Second))
	})
}

func TestGaugeSink(t *testing.T) {
	samples6 := []float64{1.0, 2.0, 3.0, 4.0, 10.0, 5.0}

	t.Run("add", func(t *testing.T) {
		t.Run("one value", func(t *testing.T) {
			sink := GaugeSink{}
			sink.Add(Sample{Metric: &Metric{}, Value: 1.0})
			assert.Equal(t, 1.0, sink.Value)
			assert.Equal(t, 1.0, sink.Min)
			assert.Equal(t, true, sink.minSet)
			assert.Equal(t, 1.0, sink.Max)
		})
		t.Run("values", func(t *testing.T) {
			sink := GaugeSink{}
			for _, s := range samples6 {
				sink.Add(Sample{Metric: &Metric{}, Value: s})
			}
			assert.Equal(t, 5.0, sink.Value)
			assert.Equal(t, 1.0, sink.Min)
			assert.Equal(t, true, sink.minSet)
			assert.Equal(t, 10.0, sink.Max)
		})
	})
	t.Run("calc", func(t *testing.T) {
		sink := GaugeSink{}
		sink.Calc()
		assert.Equal(t, 0.0, sink.Value)
		assert.Equal(t, 0.0, sink.Min)
		assert.Equal(t, false, sink.minSet)
		assert.Equal(t, 0.0, sink.Max)
	})
	t.Run("format", func(t *testing.T) {
		sink := GaugeSink{}
		for _, s := range samples6 {
			sink.Add(Sample{Metric: &Metric{}, Value: s})
		}
		assert.Equal(t, map[string]float64{"value": 5.0}, sink.Format(0))
	})
}

func TestTrendSink(t *testing.T) {
	unsortedSamples5 := []float64{0.0, 5.0, 10.0, 3.0, 1.0}
	unsortedSamples10 := []float64{0.0, 100.0, 30.0, 80.0, 70.0, 60.0, 50.0, 40.0, 90.0, 20.0}

	t.Run("add", func(t *testing.T) {
		t.Run("one value", func(t *testing.T) {
			sink := TrendSink{}
			sink.Add(Sample{Metric: &Metric{}, Value: 7.0})
			assert.Equal(t, uint64(1), sink.Count)
			assert.Equal(t, true, sink.jumbled)
			assert.Equal(t, 7.0, sink.Min)
			assert.Equal(t, 7.0, sink.Max)
			assert.Equal(t, 7.0, sink.Avg)
			assert.Equal(t, 0.0, sink.Med) // calculated in Calc()
		})
		t.Run("values", func(t *testing.T) {
			sink := TrendSink{}
			for _, s := range unsortedSamples10 {
				sink.Add(Sample{Metric: &Metric{}, Value: s})
			}
			assert.Equal(t, uint64(len(unsortedSamples10)), sink.Count)
			assert.Equal(t, true, sink.jumbled)
			assert.Equal(t, 0.0, sink.Min)
			assert.Equal(t, 100.0, sink.Max)
			assert.Equal(t, 54.0, sink.Avg)
			assert.Equal(t, 0.0, sink.Med) // calculated in Calc()
		})
	})
	t.Run("calc", func(t *testing.T) {
		t.Run("no values", func(t *testing.T) {
			sink := TrendSink{}
			sink.Calc()
			assert.Equal(t, uint64(0), sink.Count)
			assert.Equal(t, false, sink.jumbled)
			assert.Equal(t, 0.0, sink.Med)
		})
		t.Run("odd number of samples median", func(t *testing.T) {
			sink := TrendSink{}
			for _, s := range unsortedSamples5 {
				sink.Add(Sample{Metric: &Metric{}, Value: s})
			}
			sink.Calc()
			assert.Equal(t, uint64(len(unsortedSamples5)), sink.Count)
			assert.Equal(t, false, sink.jumbled)
			assert.Equal(t, 3.0, sink.Med)
		})
		t.Run("sorted", func(t *testing.T) {
			sink := TrendSink{}
			for _, s := range unsortedSamples10 {
				sink.Add(Sample{Metric: &Metric{}, Value: s})
			}
			sink.Calc()
			assert.Equal(t, uint64(len(unsortedSamples10)), sink.Count)
			assert.Equal(t, false, sink.jumbled)
			assert.Equal(t, 55.0, sink.Med)
			assert.Equal(t, 0.0, sink.Min)
			assert.Equal(t, 100.0, sink.Max)
			assert.Equal(t, 54.0, sink.Avg)
		})
	})

	tolerance := 0.000001
	t.Run("percentile", func(t *testing.T) {
		t.Run("no values", func(t *testing.T) {
			sink := TrendSink{}
			for i := 1; i <= 100; i++ {
				assert.Equal(t, 0.0, sink.P(float64(i)/100.0))
			}
		})
		t.Run("one value", func(t *testing.T) {
			sink := TrendSink{}
			sink.Add(Sample{Metric: &Metric{}, Value: 10.0})
			for i := 1; i <= 100; i++ {
				assert.Equal(t, 10.0, sink.P(float64(i)/100.0))
			}
		})
		t.Run("two values", func(t *testing.T) {
			sink := TrendSink{}
			sink.Add(Sample{Metric: &Metric{}, Value: 5.0})
			sink.Add(Sample{Metric: &Metric{}, Value: 10.0})
			assert.Equal(t, 5.0, sink.P(0.0))
			assert.Equal(t, 7.5, sink.P(0.5))
			assert.Equal(t, 5+(10-5)*0.95, sink.P(0.95))
			assert.Equal(t, 5+(10-5)*0.99, sink.P(0.99))
			assert.Equal(t, 10.0, sink.P(1.0))
		})
		t.Run("more than 2", func(t *testing.T) {
			sink := TrendSink{}
			for _, s := range unsortedSamples10 {
				sink.Add(Sample{Metric: &Metric{}, Value: s})
			}
			assert.InDelta(t, 0.0, sink.P(0.0), tolerance)
			assert.InDelta(t, 55.0, sink.P(0.5), tolerance)
			assert.InDelta(t, 95.5, sink.P(0.95), tolerance)
			assert.InDelta(t, 99.1, sink.P(0.99), tolerance)
			assert.InDelta(t, 100.0, sink.P(1.0), tolerance)
		})
	})
	t.Run("format", func(t *testing.T) {
		sink := TrendSink{}
		for _, s := range unsortedSamples10 {
			sink.Add(Sample{Metric: &Metric{}, Value: s})
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
	samples6 := []float64{1.0, 0.0, 1.0, 0.0, 0.0, 1.0}

	t.Run("add", func(t *testing.T) {
		t.Run("one true", func(t *testing.T) {
			sink := RateSink{}
			sink.Add(Sample{Metric: &Metric{}, Value: 1.0})
			assert.Equal(t, int64(1), sink.Total)
			assert.Equal(t, int64(1), sink.Trues)
		})
		t.Run("one false", func(t *testing.T) {
			sink := RateSink{}
			sink.Add(Sample{Metric: &Metric{}, Value: 0.0})
			assert.Equal(t, int64(1), sink.Total)
			assert.Equal(t, int64(0), sink.Trues)
		})
		t.Run("values", func(t *testing.T) {
			sink := RateSink{}
			for _, s := range samples6 {
				sink.Add(Sample{Metric: &Metric{}, Value: s})
			}
			assert.Equal(t, int64(6), sink.Total)
			assert.Equal(t, int64(3), sink.Trues)
		})
	})
	t.Run("calc", func(t *testing.T) {
		sink := RateSink{}
		sink.Calc()
		assert.Equal(t, int64(0), sink.Total)
		assert.Equal(t, int64(0), sink.Trues)
	})
	t.Run("format", func(t *testing.T) {
		sink := RateSink{}
		for _, s := range samples6 {
			sink.Add(Sample{Metric: &Metric{}, Value: s})
		}
		assert.Equal(t, map[string]float64{"rate": 0.5}, sink.Format(0))
	})
}

func TestDummySinkAddPanics(t *testing.T) {
	assert.Panics(t, func() {
		DummySink{}.Add(Sample{})
	})
}

func TestDummySinkCalcDoesNothing(t *testing.T) {
	sink := DummySink{"a": 1}
	sink.Calc()
	assert.Equal(t, 1.0, sink["a"])
}

func TestDummySinkFormatReturnsItself(t *testing.T) {
	assert.Equal(t, map[string]float64{"a": 1}, DummySink{"a": 1}.Format(0))
}
