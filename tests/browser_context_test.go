package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/api"
)

func TestBrowserContextAddCookies(t *testing.T) {
	t.Parallel()

	dayAfter := time.Now().
		Add(24 * time.Hour).
		Unix()
	dayBefore := time.Now().
		Add(-24 * time.Hour).
		Unix()

	tests := map[string]struct {
		name             string
		cookies          []*api.Cookie
		wantCookiesToSet []*api.Cookie
		wantErr          bool
	}{
		"cookie": {
			cookies: []*api.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
					URL:   "http://test.go",
				},
			},
			wantCookiesToSet: []*api.Cookie{
				{
					Name:     "test_cookie_name",
					Value:    "test_cookie_value",
					Domain:   "test.go",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					SameSite: "",
					Secure:   false,
				},
			},
			wantErr: false,
		},
		"cookie_with_url": {
			cookies: []*api.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
					URL:   "http://test.go",
				},
			},
			wantCookiesToSet: []*api.Cookie{
				{
					Name:     "test_cookie_name",
					Value:    "test_cookie_value",
					Domain:   "test.go",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					SameSite: "",
					Secure:   false,
				},
			},
			wantErr: false,
		},
		"cookie_with_domain_and_path": {
			cookies: []*api.Cookie{
				{
					Name:   "test_cookie_name",
					Value:  "test_cookie_value",
					Domain: "test.go",
					Path:   "/to/page",
				},
			},
			wantCookiesToSet: []*api.Cookie{
				{
					Name:     "test_cookie_name",
					Value:    "test_cookie_value",
					Domain:   "test.go",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/to/page",
					SameSite: "",
					Secure:   false,
				},
			},
			wantErr: false,
		},
		"cookie_with_expiration": {
			cookies: []*api.Cookie{
				// session cookie
				{
					Name:  "session_cookie",
					Value: "session_cookie_value",
					URL:   "http://test.go",
				},
				// persistent cookie
				{
					Name:    "persistent_cookie_name",
					Value:   "persistent_cookie_value",
					Expires: dayAfter,
					URL:     "http://test.go",
				},
				// expired cookie
				{
					Name:    "expired_cookie_name",
					Value:   "expired_cookie_value",
					Expires: dayBefore,
					URL:     "http://test.go",
				},
			},
			wantCookiesToSet: []*api.Cookie{
				{
					Name:    "session_cookie",
					Value:   "session_cookie_value",
					Domain:  "test.go",
					Expires: -1,
					Path:    "/",
				},
				{
					Name:    "persistent_cookie_name",
					Value:   "persistent_cookie_value",
					Domain:  "test.go",
					Expires: dayAfter,
					Path:    "/",
				},
			},
			wantErr: false,
		},
		"nil_cookies": {
			cookies: nil,
			wantErr: true,
		},
		"cookie_missing_name": {
			cookies: []*api.Cookie{
				{
					Value: "test_cookie_value",
					URL:   "http://test.go",
				},
			},
			wantErr: true,
		},
		"cookie_missing_value": {
			cookies: []*api.Cookie{
				{
					Name: "test_cookie_name",
					URL:  "http://test.go",
				},
			},
			wantErr: true,
		},
		"cookie_missing_url": {
			cookies: []*api.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
				},
			},
			wantErr: true,
		},
		"cookies_missing_path": {
			cookies: []*api.Cookie{
				{
					Name:   "test_cookie_name",
					Value:  "test_cookie_value",
					Domain: "test.go",
				},
			},
			wantErr: true,
		},
		"cookies_missing_domain": {
			cookies: []*api.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
					Path:  "/to/page",
				},
			},
			wantErr: true,
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			bc, err := tb.NewContext(nil)
			require.NoError(t, err)

			err = bc.AddCookies(tt.cookies)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// ensure cookies are set.
			cookies, err := bc.Cookies()
			require.NoErrorf(t,
				err, "failed to get cookies from the browser context",
			)
			require.Lenf(t,
				tt.wantCookiesToSet, len(cookies),
				"incorrect number of cookies received from the browser context",
			)
			assert.Equalf(t,
				tt.wantCookiesToSet, cookies,
				"incorrect cookies received from the browser context",
			)
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

		// addCookies is a list of cookies that will be added to the
		// browser context using the AddCookies method.
		// if empty, no cookies will be added.
		addCookies []*api.Cookie

		// filterCookiesByURLs allows to filter cookies by URLs.
		// if nil, all cookies will be returned.
		filterCookiesByURLs []string

		// wantDocumentCookies is a string representation of the
		// document.cookie value that is expected to be set.
		wantDocumentCookies string

		// wantContextCookies is a list of cookies that are expected
		// to be set in the browser context.
		wantContextCookies []*api.Cookie

		wantErr bool
	}{
		"no_cookies": {
			setupHandler: okHandler,
			documentCookiesSnippet: `
				() => {
					return document.cookie;
				}
			`,
			filterCookiesByURLs: nil,
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
			filterCookiesByURLs: nil,
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
			filterCookiesByURLs: nil,
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
			filterCookiesByURLs: nil,
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
			filterCookiesByURLs: nil,
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
			filterCookiesByURLs: nil,
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
			filterCookiesByURLs: nil,
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
			filterCookiesByURLs: nil,
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
		"filter_cookies_by_urls": {
			setupHandler: okHandler,
			documentCookiesSnippet: `
				() => {
					return document.cookie;
				}
			`,
			addCookies: []*api.Cookie{
				{
					Name:     "fooCookie",
					Value:    "fooValue",
					URL:      "https://foo.com",
					SameSite: api.CookieSameSiteNone,
				},
				{
					Name:     "barCookie",
					Value:    "barValue",
					URL:      "https://bar.com",
					SameSite: api.CookieSameSiteLax,
				},
				{
					Name:     "bazCookie",
					Value:    "bazValue",
					URL:      "https://baz.com",
					SameSite: api.CookieSameSiteLax,
				},
			},
			filterCookiesByURLs: []string{
				"https://foo.com",
				"https://baz.com",
			},
			wantDocumentCookies: "",
			wantContextCookies: []*api.Cookie{
				{
					Name:     "fooCookie",
					Value:    "fooValue",
					Domain:   "foo.com",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					Secure:   true,
					SameSite: api.CookieSameSiteNone,
				},
				{
					Name:     "bazCookie",
					Value:    "bazValue",
					Domain:   "baz.com",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					Secure:   true,
					SameSite: api.CookieSameSiteLax,
				},
			},
		},
		"filter_no_cookies": {
			setupHandler: okHandler,
			documentCookiesSnippet: `
				() => {
					return document.cookie;
				}
			`,
			addCookies: []*api.Cookie{
				{
					Name:     "fooCookie",
					Value:    "fooValue",
					URL:      "https://foo.com",
					SameSite: api.CookieSameSiteNone,
				},
				{
					Name:     "barCookie",
					Value:    "barValue",
					URL:      "https://bar.com",
					SameSite: api.CookieSameSiteLax,
				},
			},
			filterCookiesByURLs: []string{
				"https://baz.com",
			},
			wantDocumentCookies: "",
			wantContextCookies:  nil,
		},
		"filter_invalid": {
			setupHandler: okHandler,
			documentCookiesSnippet: `
				() => {
					return document.cookie;
				}
			`,
			addCookies: []*api.Cookie{
				{
					Name:     "fooCookie",
					Value:    "fooValue",
					URL:      "https://foo.com",
					SameSite: api.CookieSameSiteNone,
				},
			},
			filterCookiesByURLs: []string{
				"LOREM IPSUM",
			},
			wantDocumentCookies: "",
			wantContextCookies:  nil,
			wantErr:             true,
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

			// adding cookies to the browser context by our API.
			if tt.addCookies != nil {
				err := p.Context().AddCookies(tt.addCookies)
				require.NoErrorf(t,
					err, "failed to add cookies to the browser context: %#v", tt.addCookies,
				)
			}

			// getting cookies from the browser context
			// either from the page or from the context
			// some cookies can be set by the response handler
			cookies, err := p.Context().Cookies(tt.filterCookiesByURLs...)
			if tt.wantErr {
				require.Errorf(t,
					err, "expected an error, but got none",
				)
				return
			}
			require.NoErrorf(t,
				err, "failed to get cookies from the browser context",
			)
			assert.Lenf(t,
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
