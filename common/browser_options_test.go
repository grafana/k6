package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/k6ext/k6test"
	"github.com/grafana/xk6-browser/log"
)

func TestBrowserLaunchOptionsParse(t *testing.T) {
	t.Parallel()

	defaultOptions := &LaunchOptions{
		Env:               make(map[string]string),
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
	}

	for name, tt := range map[string]struct {
		opts            map[string]any
		assert          func(testing.TB, *LaunchOptions)
		err             string
		isRemoteBrowser bool
	}{
		"defaults": {
			opts: map[string]any{},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, defaultOptions, lo)
			},
		},
		"defaults_nil": { // providing nil option returns default options
			opts: nil,
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, defaultOptions, lo)
			},
		},
		"defaults_remote_browser": {
			isRemoteBrowser: true,
			opts: map[string]any{
				// disallow changing the following opts
				"args":              []string{"any"},
				"env":               map[string]string{"some": "thing"},
				"executablePath":    "something else",
				"headless":          false,
				"ignoreDefaultArgs": []string{"any"},
				"proxy":             ProxyOptions{Server: "srv"},
				// allow changing the following opts
				"debug":             true,
				"logCategoryFilter": "...",
				"slowMo":            time.Second,
				"timeout":           time.Second,
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(t, &LaunchOptions{
					// disallowed:
					Env:      make(map[string]string),
					Headless: true,
					// allowed:
					Debug:             true,
					LogCategoryFilter: "...",
					SlowMo:            time.Second,
					Timeout:           time.Second,

					isRemoteBrowser: true,
				}, lo)
			},
		},
		"nulls": { // don't override the defaults on `null`
			opts: map[string]any{
				"env":               nil,
				"headless":          nil,
				"logCategoryFilter": nil,
				"timeout":           nil,
			},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.Equal(tb, &LaunchOptions{
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
		"args_err": {
			opts: map[string]any{
				"args": 1,
			},
			err: "args should be an array of strings",
		},
		"debug": {
			opts: map[string]any{"debug": true},
			assert: func(tb testing.TB, lo *LaunchOptions) {
				tb.Helper()
				assert.True(t, lo.Debug)
			},
		},
		"debug_err": {
			opts: map[string]any{"debug": "true"},
			err:  "debug should be a boolean",
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
		"env_err": {
			opts: map[string]any{"env": 1},
			err:  "env should be a map",
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
		"executablePath_err": {
			opts: map[string]any{"executablePath": 1},
			err:  "executablePath should be a string",
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
		"headless_err": {
			opts: map[string]any{"headless": "true"},
			err:  "headless should be a boolean",
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
		"ignoreDefaultArgs_err": {
			opts: map[string]any{"ignoreDefaultArgs": "ABC"},
			err:  "ignoreDefaultArgs should be an array of strings",
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
		"logCategoryFilter_err": {
			opts: map[string]any{"logCategoryFilter": 1},
			err:  "logCategoryFilter should be a string",
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
		"proxy_err": {
			opts: map[string]any{"proxy": 1},
			err:  "proxy should be an object",
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
		"slowMo_err": {
			opts: map[string]any{"slowMo": "ABC"},
			err:  "slowMo should be a time duration value",
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
		"timeout_err": {
			opts: map[string]any{"timeout": "ABC"},
			err:  "timeout should be a time duration value",
		},
	} {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var (
				vu = k6test.NewVU(t)
				lo *LaunchOptions
			)

			if tt.isRemoteBrowser {
				lo = NewRemoteBrowserLaunchOptions()
			} else {
				lo = NewLaunchOptions()
			}

			err := lo.Parse(vu.Context(), log.NewNullLogger(), vu.ToGojaValue(tt.opts))
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
			} else {
				require.NoError(t, err)
				tt.assert(t, lo)
			}
		})
	}
}
