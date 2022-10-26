package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/k6ext/k6test"
)

func TestLaunchOptionsParse(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		opts   map[string]any
		assert func(testing.TB, *LaunchOptions)
	}{
		// TODO: Check other options.
		"args": {
			opts: map[string]any{
				"args": []any{"browser-arg1='value1", "browser-arg2=value2", "browser-flag"},
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				require.Len(tb, lo.Args, 3)
				assert.Equal(tb, "browser-arg1='value1", lo.Args[0])
				assert.Equal(tb, "browser-arg2=value2", lo.Args[1])
				assert.Equal(tb, "browser-flag", lo.Args[2])
			},
		},
	} {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var (
				vu = k6test.NewVU(t)
				lo = NewLaunchOptions()
			)
			require.NoError(t, lo.Parse(vu.Context(), vu.ToGojaValue(tt.opts)))
			tt.assert(t, lo)
		})
	}
}
