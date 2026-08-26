package sql

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func Test_asSymbol(t *testing.T) {
	t.Parallel()

	symbol := sobek.NewSymbol("foo")

	sym, ok := asSymbol(symbol)

	require.True(t, ok)
	require.Same(t, symbol, sym)

	rt := sobek.New()

	obj := symbol.ToObject(rt)

	sym, ok = asSymbol(obj)

	require.True(t, ok)
	require.Same(t, symbol, sym)

	sym, ok = asSymbol(sobek.Undefined())

	require.False(t, ok)
	require.Nil(t, sym)
}
