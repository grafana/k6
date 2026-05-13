package k6ext_test

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestPanicfPreservesBrowserSource(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)
	ctx := vu.Context()

	defer func() {
		v := recover()
		require.NotNil(t, v)

		obj, ok := v.(*sobek.Object)
		require.True(t, ok, "got %T, want *sobek.Object", v)

		err, ok := obj.Get("value").Export().(error)
		require.True(t, ok, "got %T, want error", obj.Get("value").Export())

		errText, fields := errext.Format(err)
		require.Equal(t, "panic boom", errText)
		require.Equal(t, map[string]any{"source": "browser"}, fields)
	}()

	k6ext.Panicf(ctx, "panic %s", "boom")
}
