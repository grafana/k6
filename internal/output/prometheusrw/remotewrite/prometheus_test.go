package remotewrite

import (
	"sort"
	"testing"
	"time"

	prompb "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/metrics"
)

// TODO: test MapSeries with suffix

func TestMapSeries(t *testing.T) {
	t.Parallel()

	r := metrics.NewRegistry()
	tags := r.RootTagSet().
		With("tagk1", "tagv1").With("b1", "v1").
		// labels with empty key or value are not allowed
		// so they will be not added as labels
		With("tagEmptyValue", "").
		With("", "tagEmptyKey")

	series := metrics.TimeSeries{
		Metric: &metrics.Metric{
			Name: "test",
			Type: metrics.Counter,
		},
		Tags: tags,
	}

	lbls := MapSeries(series, "")
	require.Len(t, lbls, 3)

	exp := []*prompb.Label{
		{Name: "__name__", Value: "k6_test"},
		{Name: "b1", Value: "v1"},
		{Name: "tagk1", Value: "tagv1"},
	}
	assert.Equal(t, exp, lbls)
}

// buildTimeSeries creates a TimSeries with the given name, value and timestamp
func buildTimeSeries(name string, value float64, timestamp time.Time) *prompb.TimeSeries { //nolint:unparam
	return &prompb.TimeSeries{
		Labels: []*prompb.Label{
			{
				Name:  "__name__",
				Value: name,
			},
		},
		Samples: []*prompb.Sample{
			{
				Value:     value,
				Timestamp: timestamp.UnixMilli(),
			},
		},
	}
}

// assertTimeSeriesMatch asserts if the elements of two slices of TimeSeries matches.
func assertTimeSeriesEqual(t *testing.T, expected []*prompb.TimeSeries, actual []*prompb.TimeSeries) {
	t.Helper()
	require.Len(t, actual, len(expected))

	for i := 0; i < len(expected); i++ {
		assert.Equal(t, expected[i], actual[i])
	}
}

// sortByLabelName sorts a slice of time series by Name label.
//
// TODO: remove the assumption that Name label is the first.
func sortByNameLabel(s []*prompb.TimeSeries) {
	sort.Slice(s, func(i, j int) bool {
		return s[i].Labels[0].Value <= s[j].Labels[0].Value
	})
}
