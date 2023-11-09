package metrics

import (
	"strconv"
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
		"a":            false, // too short
		"special\n\t":  false,
		// this has both hangul and japanese numerals .
		"hello.World_in_한글一안녕一세상": false,
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

func TestRegistryBranchTagSetRootWith(t *testing.T) {
	t.Parallel()

	raw := map[string]string{
		"key":  "val",
		"key2": "val2",
	}

	r := NewRegistry()
	tags := r.RootTagSet().WithTagsFromMap(raw)
	require.NotNil(t, tags)

	assert.Equal(t, raw, tags.Map())
}

func TestRegistryAll(t *testing.T) {
	t.Parallel()

	t.Run("Empty", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()
		assert.Nil(t, r.All())
	})

	t.Run("MultipleItems", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()

		exp := make([]string, 5)
		for i := 1; i <= 5; i++ {
			name := "metric" + strconv.Itoa(i)
			_, err := r.NewMetric(name, Counter)
			require.NoError(t, err)

			exp[i-1] = name
		}
		metrics := r.All()
		require.Len(t, metrics, 5)

		names := func(m []*Metric) []string {
			s := make([]string, len(m))
			for i := 0; i < len(m); i++ {
				s[i] = m[i].Name
			}
			return s
		}
		assert.ElementsMatch(t, exp, names(metrics))
	})
}
