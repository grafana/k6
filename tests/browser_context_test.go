package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/api"
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
		`, testCookieName, testCookieValue, tb.url(""))
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
		shouldFail  bool
	}{
		{
			description: "nil_cookies",
			cookiesCmd:  "",
			shouldFail:  true,
		},
		{
			description: "goja_null_cookies",
			cookiesCmd:  "null;",
			shouldFail:  true,
		},
		{
			description: "goja_undefined_cookies",
			cookiesCmd:  "undefined;",
			shouldFail:  true,
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
			shouldFail: true,
		},
		{
			description: "goja_cookies_string",
			cookiesCmd:  `"test_cookie_name=test_cookie_value"`,
			shouldFail:  true,
		},
		{
			description: "cookie_missing_name",
			cookiesCmd: `[
				{
					value: "test_cookie_value",
					url: "http://test.go",
				}
			];`,
			shouldFail: true,
		},
		{
			description: "cookie_missing_value",
			cookiesCmd: `[
				{
					name: "test_cookie_name",
					url: "http://test.go",
				}
			];`,
			shouldFail: true,
		},
		{
			description: "cookie_missing_url",
			cookiesCmd: `[
				{
					name: "test_cookie_name",
					value: "test_cookie_value",
				}
			];`,
			shouldFail: true,
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
			shouldFail: true,
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
			shouldFail: true,
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
			shouldFail: false,
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
			shouldFail: false,
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

			if tt.shouldFail {
				require.Error(t, bc.AddCookies(cookies))
				return
			}
			require.NoError(t, bc.AddCookies(cookies))
		})
	}
}

func TestBrowserContextCookies(t *testing.T) {
	t.Parallel()

	// an empty page is required to set cookies. we're just using a
	// simple handler that returns 200 OK to have an empty page.
	okHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	tests := map[string]struct {
		// setupHandler is a handler that will be used to setup the
		// test environment. it acts like a page returning cookies.
		setupHandler func(w http.ResponseWriter, r *http.Request)

		// documentCookiesSnippet is a JavaScript snippet that will be
		// evaluated in the page to set document.cookie.
		documentCookiesSnippet string

		// wantDocumentCookies is a string representation of the
		// document.cookie value that is expected to be set.
		wantDocumentCookies string

		// wantContextCookies is a list of cookies that are expected
		// to be set in the browser context.
		wantContextCookies []*api.Cookie
	}{
		"no_cookies": {
			setupHandler: okHandler,
			documentCookiesSnippet: `
				() => {
					return document.cookie;
				}
			`,
			wantDocumentCookies: "",
			wantContextCookies:  nil,
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// an empty page is required to set cookies
			// since we want to run cookie tests in parallel
			// we're creating a new browser context for each test.
			tb := newTestBrowser(t, withHTTPServer())
			p := tb.NewPage(nil)

			// the setupHandler can set some cookies
			// that will be received by the browser context.
			tb.withHandler("/empty", tt.setupHandler)
			_, err := p.Goto(tb.url("/empty"), nil)
			require.NoErrorf(t,
				err, "failed to open an empty page",
			)

			// setting document.cookie into the page
			cookie := p.Evaluate(tb.toGojaValue(tt.documentCookiesSnippet))
			require.Equalf(t,
				tt.wantDocumentCookies,
				tb.asGojaValue(cookie).String(),
				"incorrect document.cookie received",
			)

			// getting cookies from the browser context
			// either from the page or from the context
			// some cookies can be set by the response handler
			cookies, err := p.Context().Cookies()
			require.NoErrorf(t,
				err, "failed to get cookies from the browser context",
			)
			require.Lenf(t,
				cookies, len(tt.wantContextCookies),
				"incorrect number of cookies received from the browser context",
			)
			assert.Equalf(t,
				tt.wantContextCookies, cookies,
				"incorrect cookies received from the browser context",
			)
		})
	}
}
