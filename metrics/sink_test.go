package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCounterSinkAddSample(t *testing.T) {
	t.Parallel()
	samples10 := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 100.0}
	now := time.Now()

	t.Run("one value", func(t *testing.T) {
		t.Parallel()
		sink := CounterSink{}
		sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 1.0, Time: now})
		// assert.Equal(t, now, sink.first)
		assert.Equal(t, 1.0, sink.LastValue())
	})
	t.Run("values", func(t *testing.T) {
		t.Parallel()
		sink := CounterSink{}
		for _, s := range samples10 {
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s, Time: now})
		}
		assert.Equal(t, 145.0, sink.LastValue())
		// assert.Equal(t, now, sink.First)
	})
}

func TestCounterSinkRate(t *testing.T) {
	t.Parallel()
	sink := CounterSink{}

	samples10 := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 100.0}
	for _, s := range samples10 {
		sink.Add(Sample{
			TimeSeries: TimeSeries{Metric: &Metric{}},
			Value:      s,
			Time:       time.Now(),
		})
	}

	assert.Equal(t, 0.0, sink.Rate(0))               // returns 0 if duration is zero
	assert.Equal(t, 145.0, sink.Rate(1*time.Second)) // returns the rate if duration is greater
}

func TestGaugeSinkAddSample(t *testing.T) {
	t.Parallel()
	samples6 := []float64{1.0, 2.0, 3.0, 4.0, 10.0, 5.0}

	t.Run("one value", func(t *testing.T) {
		t.Parallel()
		sink := GaugeSink{}
		sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 1.0})
		assert.Equal(t, 1.0, sink.LastValue())
		assert.Equal(t, 1.0, sink.Min())
		// assert.Equal(t, true, sink.minSet)
		assert.Equal(t, 1.0, sink.Max())
	})
	t.Run("values", func(t *testing.T) {
		t.Parallel()
		sink := GaugeSink{}
		for _, s := range samples6 {
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
		}
		assert.Equal(t, 5.0, sink.LastValue())
		assert.Equal(t, 1.0, sink.Min())
		assert.Equal(t, true, sink.minSet)
		assert.Equal(t, 10.0, sink.Max())
	})
}

func TestTrendSinkAddSample(t *testing.T) {
	t.Parallel()
	unsortedSamples10 := []float64{0.0, 100.0, 30.0, 80.0, 70.0, 60.0, 50.0, 40.0, 90.0, 20.0}

	t.Run("one value", func(t *testing.T) {
		t.Parallel()
		sink := TrendSink{}
		sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 7.0})
		assert.Equal(t, uint64(1), sink.Count())
		assert.Equal(t, false, sink.sorted)
		assert.Equal(t, 7.0, sink.Min())
		assert.Equal(t, 7.0, sink.Max())
		assert.Equal(t, 7.0, sink.Avg())
	})
	t.Run("values", func(t *testing.T) {
		t.Parallel()
		sink := TrendSink{}
		for _, s := range unsortedSamples10 {
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
		}
		assert.Equal(t, uint64(len(unsortedSamples10)), sink.Count())
		// assert.Equal(t, false, sink.sorted)
		assert.Equal(t, 0.0, sink.Min())
		assert.Equal(t, 100.0, sink.Max())
		assert.Equal(t, 54.0, sink.Avg())
	})
}

func TestTrendSinkP(t *testing.T) {
	t.Parallel()
	tolerance := 0.000001
	unsortedSamples10 := []float64{0.0, 100.0, 30.0, 80.0, 70.0, 60.0, 50.0, 40.0, 90.0, 20.0}

	t.Run("no values", func(t *testing.T) {
		t.Parallel()
		sink := TrendSink{}
		for i := 1; i <= 100; i++ {
			assert.Equal(t, 0.0, sink.P(float64(i)/100.0))
		}
	})
	t.Run("one value", func(t *testing.T) {
		t.Parallel()
		sink := TrendSink{}
		sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 10.0})
		for i := 1; i <= 100; i++ {
			assert.Equal(t, 10.0, sink.P(float64(i)/100.0))
		}
	})
	t.Run("two values", func(t *testing.T) {
		t.Parallel()
		sink := TrendSink{}
		sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 5.0})
		sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 10.0})
		assert.Equal(t, false, sink.sorted)
		assert.Equal(t, 5.0, sink.P(0.0))
		assert.Equal(t, 7.5, sink.P(0.5))
		assert.Equal(t, 5+(10-5)*0.95, sink.P(0.95))
		assert.Equal(t, 5+(10-5)*0.99, sink.P(0.99))
		assert.Equal(t, 10.0, sink.P(1.0))
		assert.Equal(t, true, sink.sorted)
	})
	t.Run("more than 2", func(t *testing.T) {
		t.Parallel()
		sink := TrendSink{}
		for _, s := range unsortedSamples10 {
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
		}
		assert.InDelta(t, 0.0, sink.P(0.0), tolerance)
		assert.InDelta(t, 55.0, sink.P(0.5), tolerance)
		assert.InDelta(t, 95.5, sink.P(0.95), tolerance)
		assert.InDelta(t, 99.1, sink.P(0.99), tolerance)
		assert.InDelta(t, 100.0, sink.P(1.0), tolerance)
		assert.Equal(t, true, sink.sorted)
	})
}

func TestRateSinkAddSample(t *testing.T) {
	t.Parallel()
	samples6 := []float64{1.0, 0.0, 1.0, 0.0, 0.0, 1.0}

	t.Run("one true", func(t *testing.T) {
		t.Parallel()
		sink := RateSink{}
		sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 1.0})
		assert.Equal(t, 1.0, sink.Rate())
	})
	t.Run("one false", func(t *testing.T) {
		t.Parallel()
		sink := RateSink{}
		sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: 0.0})
		assert.Equal(t, 0.0, sink.Rate())
	})
	t.Run("values", func(t *testing.T) {
		t.Parallel()
		sink := RateSink{}
		for _, s := range samples6 {
			sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
		}
		assert.Equal(t, int64(6), sink.total)
		assert.Equal(t, int64(3), sink.trues)
	})
}

func TestRateSinkRate(t *testing.T) {
	t.Parallel()
	samples6 := []float64{1.0, 0.0, 1.0, 0.0, 0.0, 1.0}
	sink := RateSink{}
	for _, s := range samples6 {
		sink.Add(Sample{TimeSeries: TimeSeries{Metric: &Metric{}}, Value: s})
	}
	assert.Equal(t, 0.5, sink.Rate())
}

func TestDummySinkAddPanics(t *testing.T) {
	assert.Panics(t, func() {
		DummySink{}.Add(Sample{})
	})
}
