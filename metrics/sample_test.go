package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSampleImplementations(t *testing.T) {
	r := NewRegistry()
	sampleTags := r.RootTagSet().With("key1", "val1").With("key2", "val2")
	now := time.Now()

	sample := Sample{
		TimeSeries: TimeSeries{
			Metric: r.MustNewMetric("test_metric", Counter),
			Tags:   sampleTags,
		},
		Time:  now,
		Value: 1.0,
	}
	samples := Samples(sample.GetSamples())
	cSamples := ConnectedSamples{
		Samples: []Sample{sample},
		Time:    now,
		Tags:    sampleTags,
	}
	exp := []Sample{sample}
	assert.Equal(t, exp, sample.GetSamples())
	assert.Equal(t, exp, samples.GetSamples())
	assert.Equal(t, exp, cSamples.GetSamples())
	assert.Equal(t, now, sample.GetTime())
	assert.Equal(t, now, cSamples.GetTime())
	assert.Equal(t, sample.GetTags(), cSamples.GetTags())
}

func TestGetResolversForTrendColumnsValidation(t *testing.T) {
	validateTests := []struct {
		stats  []string
		expErr bool
	}{
		{[]string{}, false},
		{[]string{"avg", "min", "med", "max", "p(0)", "p(99)", "p(99.999)", "count"}, false},
		{[]string{"avg", "p(err)"}, true},
		{[]string{"nil", "p(err)"}, true},
		{[]string{"p90"}, true},
		{[]string{"p(90"}, true},
		{[]string{" avg"}, true},
		{[]string{"avg "}, true},
		{[]string{"", "avg "}, true},
		{[]string{"p(-1)"}, true},
		{[]string{"p(101)"}, true},
		{[]string{"p(1)"}, false},
	}

	for _, tc := range validateTests {
		tc := tc
		t.Run(fmt.Sprintf("%v", tc.stats), func(t *testing.T) {
			_, err := GetResolversForTrendColumns(tc.stats)
			if tc.expErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetResolversForTrendColumnsCalculation(t *testing.T) {
	t.Parallel()

	customResolversTests := []struct {
		stats      string
		percentile float64
	}{
		{"p(50)", 0.5},
		{"p(99)", 0.99},
		{"p(99.9)", 0.999},
		{"p(99.99)", 0.9999},
		{"p(99.999)", 0.99999},
	}

	for _, tc := range customResolversTests {
		tc := tc
		t.Run(fmt.Sprintf("%v", tc.stats), func(t *testing.T) {
			t.Parallel()
			sink := createTestTrendSink(100)

			res, err := GetResolversForTrendColumns([]string{tc.stats})
			assert.NoError(t, err)
			assert.Len(t, res, 1)
			for k := range res {
				assert.InDelta(t, sink.P(tc.percentile), res[k](sink), 0.000001)
			}
		})
	}
}

func createTestTrendSink(count int) *TrendSink {
	sink := NewTrendSink()

	for i := 0; i < count; i++ {
		sink.Add(Sample{Value: float64(i)})
	}

	return sink
}
