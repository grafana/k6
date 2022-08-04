package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryNewMetric(t *testing.T) {
	t.Parallel()
	r := NewRegistry()

	somethingCounter, err := r.NewMetric("something", Counter)
	require.NoError(t, err)
	require.Equal(t, "something", somethingCounter.Name)

	somethingCounterAgain, err := r.NewMetric("something", Counter)
	require.NoError(t, err)
	require.Equal(t, "something", somethingCounterAgain.Name)
	require.Same(t, somethingCounter, somethingCounterAgain)

	_, err = r.NewMetric("something", Gauge)
	require.Error(t, err)

	_, err = r.NewMetric("something", Counter, Time)
	require.Error(t, err)
}

func TestMetricNames(t *testing.T) {
	t.Parallel()
	testMap := map[string]bool{
		"simple":       true,
		"still_simple": true,
		"":             false,
		"@":            false,
		"a":            true,
		"special\n\t":  false,
		// this has both hangul and japanese numerals .
		"hello.World_in_한글一안녕一세상": true,
		// too long
		"tooolooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooog": false,
	}

	for key, value := range testMap {
		key, value := key, value
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, value, checkName(key), key)
		})
	}
}
