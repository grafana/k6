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

		const (
			testCookieName  = "test_cookie_name"
			testCookieValue = "test_cookie_value"
		)

		bc, err := tb.NewContext(nil)
		require.NoError(t, err)
		err = bc.AddCookies([]*api.Cookie{
			{
				Name:  testCookieName,
				Value: testCookieValue,
				URL:   tb.url(""),
			},
		})
		require.NoError(t, err)

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
		cookies     []*api.Cookie
		shouldFail  bool
	}{
		{
			description: "nil_cookies",
			cookies:     nil,
			shouldFail:  true,
		},
		{
			description: "cookie",
			cookies: []*api.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
					URL:   "http://test.go",
				},
			},
			shouldFail: false,
		},
		{
			description: "cookie_missing_name",
			cookies: []*api.Cookie{
				{
					Value: "test_cookie_value",
					URL:   "http://test.go",
				},
			},
			shouldFail: true,
		},
		{
			description: "cookie_missing_value",
			cookies: []*api.Cookie{
				{
					Name: "test_cookie_name",
					URL:  "http://test.go",
				},
			},
			shouldFail: true,
		},
		{
			description: "cookie_missing_url",
			cookies: []*api.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
				},
			},
			shouldFail: true,
		},
		{
			description: "cookies_missing_path",
			cookies: []*api.Cookie{
				{
					Name:   "test_cookie_name",
					Value:  "test_cookie_value",
					Domain: "test.go",
				},
			},
			shouldFail: true,
		},
		{
			description: "cookies_missing_domain",
			cookies: []*api.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
					Path:  "/to/page",
				},
			},
			shouldFail: true,
		},

		{
			description: "cookie_with_url",
			cookies: []*api.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
					URL:   "http://test.go",
				},
			},
			shouldFail: false,
		},
		{
			description: "cookie_with_domain_and_path",
			cookies: []*api.Cookie{
				{
					Name:   "test_cookie_name",
					Value:  "test_cookie_value",
					Domain: "test.go",
					Path:   "/to/page",
				},
			},
			shouldFail: false,
		},
	}
	for _, tt := range errorTests {
		tt := tt
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			bc, err := tb.NewContext(nil)
			require.NoError(t, err)

			err = bc.AddCookies(tt.cookies)
			if tt.shouldFail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
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
		"cookie": {
			setupHandler: okHandler,
			documentCookiesSnippet: `
				() => {
					document.cookie = "name=value";
					return document.cookie;
				}
			`,
			wantDocumentCookies: "name=value",
			wantContextCookies: []*api.Cookie{
				{
					Name:     "name",
					Value:    "value",
					Domain:   "127.0.0.1",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					SameSite: "",
					Secure:   false,
				},
			},
		},
		"cookies": {
			setupHandler: okHandler,
			documentCookiesSnippet: `
				() => {
					document.cookie = "name=value";
					document.cookie = "name2=value2";
					return document.cookie;
				}
			`,
			wantDocumentCookies: "name=value; name2=value2",
			wantContextCookies: []*api.Cookie{
				{
					Name:     "name",
					Value:    "value",
					Domain:   "127.0.0.1",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					SameSite: "",
					Secure:   false,
				},
				{
					Name:     "name2",
					Value:    "value2",
					Domain:   "127.0.0.1",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					SameSite: "",
					Secure:   false,
				},
			},
		},
		"cookie_with_path": {
			setupHandler: okHandler,
			documentCookiesSnippet: `
				() => {
					document.cookie = "name=value; path=/empty";
					return document.cookie;
				}
			`,
			wantDocumentCookies: "name=value",
			wantContextCookies: []*api.Cookie{
				{
					Name:     "name",
					Value:    "value",
					Domain:   "127.0.0.1",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/empty",
					SameSite: "",
					Secure:   false,
				},
			},
		},
		"cookie_with_different_domain": {
			setupHandler: okHandler,
			documentCookiesSnippet: `
				() => {
					document.cookie = "name=value; domain=k6.io";
					return document.cookie;
				}
			`,
			wantDocumentCookies: "", // some cookies cannot be set (i.e. cookies using different domains)
			wantContextCookies:  nil,
		},
		"http_only_cookie": {
			setupHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Set-Cookie", "name=value;HttpOnly; Path=/")
			},
			documentCookiesSnippet: `
				() => {
					return document.cookie;
				}
			`,
			wantDocumentCookies: "",
			wantContextCookies: []*api.Cookie{
				{
					HTTPOnly: true,
					Name:     "name",
					Value:    "value",
					Domain:   "127.0.0.1",
					Expires:  -1,
					Path:     "/",
					SameSite: "",
					Secure:   false,
				},
			},
		},
		"same_site_strict_cookie": {
			setupHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Set-Cookie", "name=value;SameSite=Strict")
			},
			documentCookiesSnippet: `
				() => {
					return document.cookie;
				}
			`,
			wantDocumentCookies: "name=value",
			wantContextCookies: []*api.Cookie{
				{
					SameSite: api.CookieSameSiteStrict,
					Name:     "name",
					Value:    "value",
					Domain:   "127.0.0.1",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					Secure:   false,
				},
			},
		},
		"same_site_lax_cookie": {
			setupHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Set-Cookie", "name=value;SameSite=Lax")
			},
			documentCookiesSnippet: `
				() => {
					return document.cookie;
				}
			`,
			wantDocumentCookies: "name=value",
			wantContextCookies: []*api.Cookie{
				{
					SameSite: api.CookieSameSiteLax,
					Name:     "name",
					Value:    "value",
					Domain:   "127.0.0.1",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					Secure:   false,
				},
			},
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

func TestK6Object(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)

	url := b.staticURL("empty.html")
	r, err := p.Goto(url, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	k6Obj := p.Evaluate(b.toGojaValue(`() => window.k6`))
	k6ObjGoja := b.toGojaValue(k6Obj)

	assert.False(t, k6ObjGoja.Equals(goja.Null()))
}
