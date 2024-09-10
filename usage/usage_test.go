package usage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrors(t *testing.T) {
	t.Parallel()
	u := New()
	require.NoError(t, u.Uint64("test/one", 1))
	require.NoError(t, u.Uint64("test/two", 1))
	require.NoError(t, u.Uint64("test/two", 1))
	require.NoError(t, u.Strings("test/three", "three"))
	require.NoError(t, u.Strings("test2/one", "one"))

	require.ErrorContains(t, u.Strings("test/one", "one"),
		"test/one is not []string as expected but uint64")
	require.ErrorContains(t, u.Uint64("test2/one", 1),
		"test2/one is not uint64 as expected but []string")

	require.NoError(t, u.Strings("test3", "some"))
	require.NoError(t, u.Strings("test3/one", "one"))

	m, err := u.Map()
	require.ErrorContains(t, err,
		"key test3 was expected to be a map[string]any but was []string")
	require.EqualValues(t, map[string]any{
		"test": map[string]any{
			"one":   uint64(1),
			"two":   uint64(2),
			"three": []string{"three"},
		},
		"test2": map[string]any{
			"one": []string{"one"},
		},
		"test3": []string{"some"},
	}, m)
}
