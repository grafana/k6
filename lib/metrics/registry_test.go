package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/stats"
)

func TestRegistryNewMetric(t *testing.T) {
	t.Parallel()
	r := NewRegistry()

	somethingCounter, err := r.NewMetric("something", stats.Counter)
	require.NoError(t, err)
	require.Equal(t, "something", somethingCounter.Name)

	somethingCounterAgain, err := r.NewMetric("something", stats.Counter)
	require.NoError(t, err)
	require.Equal(t, "something", somethingCounterAgain.Name)
	require.Same(t, somethingCounter, somethingCounterAgain)

	_, err = r.NewMetric("something", stats.Gauge)
	require.Error(t, err)

	_, err = r.NewMetric("something", stats.Counter, stats.Time)
	require.Error(t, err)
}
