package faker_test

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/require"
	"lukechampine.com/frand"
)

func Test_lookup(t *testing.T) {
	t.Parallel()

	funcs := []string{"creditcardstring", "creditcardexpmonth", "creditcardexpyear"}

	for _, fun := range funcs {
		t.Run(fun, func(t *testing.T) {
			t.Parallel()

			info := gofakeit.GetFuncLookup(fun)

			require.NotNil(t, info)
		})
	}
}

func testRand(t *testing.T) *rand.Rand {
	t.Helper()

	src := frand.NewSource()
	src.Seed(11)

	return rand.New(src)
}

func Test_creditcardexpyear(t *testing.T) {
	t.Parallel()

	info := gofakeit.GetFuncLookup("creditcardexpyear")

	require.NotNil(t, info)

	val, err := info.Generate(testRand(t), nil, info)

	require.NoError(t, err)
	require.Len(t, val, 2)

	str, ok := val.(string)
	require.True(t, ok)

	year, err := strconv.Atoi(str)

	require.NoError(t, err)

	require.Greater(t, year, time.Now().Year()-2000)
}

func Test_creditcardexpmonth(t *testing.T) {
	t.Parallel()

	info := gofakeit.GetFuncLookup("creditcardexpmonth")

	require.NotNil(t, info)

	val, err := info.Generate(testRand(t), nil, info)

	require.NoError(t, err)
	require.Len(t, val, 2)

	str, ok := val.(string)
	require.True(t, ok)

	month, err := strconv.Atoi(str)

	require.NoError(t, err)

	require.Positive(t, month)
	require.Less(t, month, 13)
}

func Test_creditcardstring(t *testing.T) {
	t.Parallel()

	info := gofakeit.GetFuncLookup("creditcardstring")

	require.NotNil(t, info)

	val, err := info.Generate(testRand(t), nil, info)

	require.NoError(t, err)
	require.Equal(t, "4111-1111-1111-1111", val)
}
