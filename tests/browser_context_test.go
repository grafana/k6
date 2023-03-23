package tests

import (
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserContextAddCookies(t *testing.T) {
	t.Parallel()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())

		testCookieName := "test_cookie_name"
		testCookieValue := "test_cookie_value"

		bc, err := tb.NewContext(nil)
		require.NoError(t, err)
		cookies, err := tb.runJavaScript(`
			[
				{
					name: "%v",
					value: "%v",
					url: "%v"
				}
			];
		`, testCookieName, testCookieValue, tb.URL(""))
		require.NoError(t, err)

		bc.AddCookies(cookies)

		p, err := bc.NewPage()
		require.NoError(t, err)

		_, err = p.Goto(
			tb.staticURL("add_cookies.html"),
			tb.toGojaValue(struct {
				WaitUntil string `js:"waitUntil"`
			}{
				WaitUntil: "load",
			}),
		)
		require.NoError(t, err)

		result := p.TextContent("#cookies", nil)
		assert.EqualValues(t, fmt.Sprintf("%v=%v", testCookieName, testCookieValue), result)
	})

	errorTests := []struct {
		description string
		cookiesCmd  string
		shouldPanic bool
	}{
		{
			description: "nil_cookies",
			cookiesCmd:  "",
			shouldPanic: true,
		},
		{
			description: "goja_null_cookies",
			cookiesCmd:  "null;",
			shouldPanic: true,
		},
		{
			description: "goja_undefined_cookies",
			cookiesCmd:  "undefined;",
			shouldPanic: true,
		},
		{
			description: "goja_cookies_object",
			cookiesCmd: `
				({
					name: "test_cookie_name",
					value: "test_cookie_value",
					url: "http://test.go",
				});
			`,
			shouldPanic: true,
		},
		{
			description: "goja_cookies_string",
			cookiesCmd:  `"test_cookie_name=test_cookie_value"`,
			shouldPanic: true,
		},
		{
			description: "cookie_missing_name",
			cookiesCmd: `[
				{
					value: "test_cookie_value",
					url: "http://test.go",
				}
			];`,
			shouldPanic: true,
		},
		{
			description: "cookie_missing_value",
			cookiesCmd: `[
				{
					name: "test_cookie_name",
					url: "http://test.go",
				}
			];`,
			shouldPanic: true,
		},
		{
			description: "cookie_missing_url",
			cookiesCmd: `[
				{
					name: "test_cookie_name",
					value: "test_cookie_value",
				}
			];`,
			shouldPanic: true,
		},
		{
			description: "cookies_missing_path",
			cookiesCmd: `[
				{
					name: "test_cookie_name",
					value: "test_cookie_value",
					domain: "http://test.go",
				}
			];`,
			shouldPanic: true,
		},
		{
			description: "cookies_missing_domain",
			cookiesCmd: `[
				{
					name: "test_cookie_name",
					value: "test_cookie_value",
					path: "/to/page",
				}
			];`,
			shouldPanic: true,
		},
		{
			description: "cookie_with_url",
			cookiesCmd: `[
				{
					name: "test_cookie_name",
					value: "test_cookie_value",
					url: "http://test.go",
				}
			];`,
			shouldPanic: false,
		},
		{
			description: "cookie_with_domain_and_path",
			cookiesCmd: `[
				{
					name: "test_cookie_name",
					value: "test_cookie_value",
					domain: "http://test.go",
					path: "/to/page",
				}
			];`,
			shouldPanic: false,
		},
	}

	for _, tt := range errorTests {
		tt := tt
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())

			var cookies goja.Value
			if tt.cookiesCmd != "" {
				var err error
				cookies, err = tb.runJavaScript(tt.cookiesCmd)
				require.NoError(t, err)
			}

			bc, err := tb.NewContext(nil)
			require.NoError(t, err)

			if tt.shouldPanic {
				assert.Panics(t, func() { bc.AddCookies(cookies) })
			} else {
				assert.NotPanics(t, func() { bc.AddCookies(cookies) })
			}
		})
	}
}
