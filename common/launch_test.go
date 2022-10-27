package common

import (
	"testing"
	"time"

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
		// TODO: Check parsing errors
		"defaults": {
			opts: map[string]any{},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, &LaunchOptions{
					Env:               make(map[string]string),
					Headless:          true,
					LogCategoryFilter: ".*",
					Timeout:           DefaultTimeout,
				}, lo)
			},
		},
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
		"debug": {
			opts: map[string]any{"debug": true},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.True(t, lo.Debug)
			},
		},
		"devtools": {
			opts: map[string]any{
				"devtools": true,
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.True(t, lo.Devtools)
			},
		},
		"env": {
			opts: map[string]any{
				"env": map[string]string{"key": "value"},
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, map[string]string{"key": "value"}, lo.Env)
			},
		},
		"executablePath": {
			opts: map[string]any{
				"executablePath": "cmd/somewhere",
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, "cmd/somewhere", lo.ExecutablePath)
			},
		},
		"headless": {
			opts: map[string]any{
				"headless": false,
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.False(t, lo.Headless)
			},
		},
		"ignoreDefaultArgs": {
			opts: map[string]any{
				"ignoreDefaultArgs": []string{"--hide-scrollbars", "--hide-something"},
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Len(t, lo.IgnoreDefaultArgs, 2)
				assert.Equal(t, "--hide-scrollbars", lo.IgnoreDefaultArgs[0])
				assert.Equal(t, "--hide-something", lo.IgnoreDefaultArgs[1])
			},
		},
		"logCategoryFilter": {
			opts: map[string]any{
				"logCategoryFilter": "**",
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, "**", lo.LogCategoryFilter)
			},
		},
		"proxy": {
			opts: map[string]any{
				"proxy": ProxyOptions{
					Server:   "serverVal",
					Bypass:   "bypassVal",
					Username: "usernameVal",
					Password: "passwordVal",
				},
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, ProxyOptions{
					Server:   "serverVal",
					Bypass:   "bypassVal",
					Username: "usernameVal",
					Password: "passwordVal",
				}, lo.Proxy)
			},
		},
		"slowMo": {
			opts: map[string]any{
				"slowMo": "5s",
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, 5*time.Second, lo.SlowMo)
			},
		},
		"timeout": {
			opts: map[string]any{
				"timeout": "10s",
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, 10*time.Second, lo.Timeout)
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
