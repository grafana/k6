package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

func TestBrowserOptionsParse(t *testing.T) {
	t.Parallel()

	defaultOptions := &BrowserOptions{
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
	}

	for name, tt := range map[string]struct {
		opts            map[string]any
		envLookupper    env.LookupFunc
		assert          func(testing.TB, *BrowserOptions)
		err             string
		isRemoteBrowser bool
	}{
		"defaults": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.EmptyLookup,
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.Equal(t, defaultOptions, lo)
			},
		},
		"defaults_nil": { // providing nil option returns default options
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.EmptyLookup,
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.Equal(t, defaultOptions, lo)
			},
		},
		"defaults_remote_browser": {
			opts: map[string]any{
				"type": "chromium",
			},
			isRemoteBrowser: true,
			envLookupper: func(k string) (string, bool) {
				switch k {
				// disallow changing the following opts
				case env.BrowserArguments:
					return "any", true
				case env.BrowserExecutablePath:
					return "something else", true
				case env.BrowserHeadless:
					return "false", true
				case env.BrowserIgnoreDefaultArgs:
					return "any", true
				// allow changing the following opts
				case env.BrowserEnableDebugging:
					return "true", true
				case env.LogCategoryFilter:
					return "...", true
				case env.BrowserGlobalTimeout:
					return "1s", true
				default:
					return "", false
				}
			},
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.Equal(t, &BrowserOptions{
					// disallowed:
					Headless: true,
					// allowed:
					Debug:             true,
					LogCategoryFilter: "...",
					Timeout:           time.Second,

					isRemoteBrowser: true,
				}, lo)
			},
		},
		"nulls": { // don't override the defaults on `null`
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: func(k string) (string, bool) {
				return "", true
			},
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.Equal(tb, &BrowserOptions{
					Headless:          true,
					LogCategoryFilter: ".*",
					Timeout:           DefaultTimeout,
				}, lo)
			},
		},
		"args": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.BrowserArguments, "browser-arg1='value1,browser-arg2=value2,browser-flag"),
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				require.Len(tb, lo.Args, 3)
				assert.Equal(tb, "browser-arg1='value1", lo.Args[0])
				assert.Equal(tb, "browser-arg2=value2", lo.Args[1])
				assert.Equal(tb, "browser-flag", lo.Args[2])
			},
		},
		"debug": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.BrowserEnableDebugging, "true"),
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.True(t, lo.Debug)
			},
		},
		"debug_err": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.BrowserEnableDebugging, "non-boolean"),
			err:          "K6_BROWSER_DEBUG should be a boolean",
		},
		"executablePath": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.BrowserExecutablePath, "cmd/somewhere"),
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.Equal(t, "cmd/somewhere", lo.ExecutablePath)
			},
		},
		"headless": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.BrowserHeadless, "false"),
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.False(t, lo.Headless)
			},
		},
		"headless_err": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.BrowserHeadless, "non-boolean"),
			err:          "K6_BROWSER_HEADLESS should be a boolean",
		},
		"ignoreDefaultArgs": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.BrowserIgnoreDefaultArgs, "--hide-scrollbars,--hide-something"),
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.Len(t, lo.IgnoreDefaultArgs, 2)
				assert.Equal(t, "--hide-scrollbars", lo.IgnoreDefaultArgs[0])
				assert.Equal(t, "--hide-something", lo.IgnoreDefaultArgs[1])
			},
		},
		"logCategoryFilter": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.LogCategoryFilter, "**"),
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.Equal(t, "**", lo.LogCategoryFilter)
			},
		},
		"timeout": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.BrowserGlobalTimeout, "10s"),
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				assert.Equal(t, 10*time.Second, lo.Timeout)
			},
		},
		"timeout_err": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.ConstLookup(env.BrowserGlobalTimeout, "ABC"),
			err:          "K6_BROWSER_TIMEOUT should be a time duration value",
		},
		"browser_type": {
			opts: map[string]any{
				"type": "chromium",
			},
			envLookupper: env.EmptyLookup,
			assert: func(tb testing.TB, lo *BrowserOptions) {
				tb.Helper()
				// Noop, just expect no error
			},
		},
		"browser_type_err": {
			opts: map[string]any{
				"type": "mybrowsertype",
			},
			envLookupper: env.EmptyLookup,
			err:          "unsupported browser type: mybrowsertype",
		},
		"browser_type_unset_err": {
			envLookupper: env.EmptyLookup,
			err:          "browser type option must be set",
		},
	} {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var (
				vu = k6test.NewVU(t)
				lo *BrowserOptions
			)

			if tt.isRemoteBrowser {
				lo = NewRemoteBrowserOptions()
			} else {
				lo = NewLocalBrowserOptions()
			}

			err := lo.Parse(vu.Context(), log.NewNullLogger(), tt.opts, tt.envLookupper)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
			} else {
				require.NoError(t, err)
				tt.assert(t, lo)
			}
		})
	}
}
