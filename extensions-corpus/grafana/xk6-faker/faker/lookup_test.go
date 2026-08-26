package faker_test

import (
	"testing"

	"github.com/grafana/xk6-faker/faker"
	"github.com/stretchr/testify/require"
)

func TestGetFuncLookups(t *testing.T) {
	t.Parallel()

	funcs := faker.GetFuncLookups()

	require.Len(t, funcs, 303)
	require.Contains(t, funcs, "intRange")
	require.Contains(t, funcs, "randomString")
}

func TestGetCategoryFuncs(t *testing.T) {
	t.Parallel()

	categories := faker.GetCategoryFuncs()

	require.Len(t, categories, 29)
	require.Contains(t, categories, "zen")
	require.Contains(t, categories, "numbers")

	require.Contains(t, categories["zen"], "intRange")
	require.Contains(t, categories["numbers"], "intRange")
	require.Same(t, categories["zen"]["intRange"], categories["numbers"]["intRange"])
}
