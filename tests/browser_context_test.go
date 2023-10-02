package tests

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
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
		cookies          []*common.Cookie
		wantCookiesToSet []*common.Cookie
		wantErr          bool
	}{
		"cookie": {
			cookies: []*common.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
					URL:   "http://test.go",
				},
			},
			wantCookiesToSet: []*common.Cookie{
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
			cookies: []*common.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
					URL:   "http://test.go",
				},
			},
			wantCookiesToSet: []*common.Cookie{
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
			cookies: []*common.Cookie{
				{
					Name:   "test_cookie_name",
					Value:  "test_cookie_value",
					Domain: "test.go",
					Path:   "/to/page",
				},
			},
			wantCookiesToSet: []*common.Cookie{
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
			cookies: []*common.Cookie{
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
			wantCookiesToSet: []*common.Cookie{
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
			cookies: []*common.Cookie{
				{
					Value: "test_cookie_value",
					URL:   "http://test.go",
				},
			},
			wantErr: true,
		},
		"cookie_missing_value": {
			cookies: []*common.Cookie{
				{
					Name: "test_cookie_name",
					URL:  "http://test.go",
				},
			},
			wantErr: true,
		},
		"cookie_missing_url": {
			cookies: []*common.Cookie{
				{
					Name:  "test_cookie_name",
					Value: "test_cookie_value",
				},
			},
			wantErr: true,
		},
		"cookies_missing_path": {
			cookies: []*common.Cookie{
				{
					Name:   "test_cookie_name",
					Value:  "test_cookie_value",
					Domain: "test.go",
				},
			},
			wantErr: true,
		},
		"cookies_missing_domain": {
			cookies: []*common.Cookie{
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
		addCookies []*common.Cookie

		// filterCookiesByURLs allows to filter cookies by URLs.
		// if nil, all cookies will be returned.
		filterCookiesByURLs []string

		// wantDocumentCookies is a string representation of the
		// document.cookie value that is expected to be set.
		wantDocumentCookies string

		// wantContextCookies is a list of cookies that are expected
		// to be set in the browser context.
		wantContextCookies []*common.Cookie

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
			wantContextCookies: []*common.Cookie{
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
			wantContextCookies: []*common.Cookie{
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
			wantContextCookies: []*common.Cookie{
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
			wantContextCookies: []*common.Cookie{
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
			wantContextCookies: []*common.Cookie{
				{
					SameSite: common.CookieSameSiteStrict,
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
			wantContextCookies: []*common.Cookie{
				{
					SameSite: common.CookieSameSiteLax,
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
			addCookies: []*common.Cookie{
				{
					Name:     "fooCookie",
					Value:    "fooValue",
					URL:      "https://foo.com",
					SameSite: common.CookieSameSiteNone,
				},
				{
					Name:     "barCookie",
					Value:    "barValue",
					URL:      "https://bar.com",
					SameSite: common.CookieSameSiteLax,
				},
				{
					Name:     "bazCookie",
					Value:    "bazValue",
					URL:      "https://baz.com",
					SameSite: common.CookieSameSiteLax,
				},
			},
			filterCookiesByURLs: []string{
				"https://foo.com",
				"https://baz.com",
			},
			wantDocumentCookies: "",
			wantContextCookies: []*common.Cookie{
				{
					Name:     "fooCookie",
					Value:    "fooValue",
					Domain:   "foo.com",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					Secure:   true,
					SameSite: common.CookieSameSiteNone,
				},
				{
					Name:     "bazCookie",
					Value:    "bazValue",
					Domain:   "baz.com",
					Expires:  -1,
					HTTPOnly: false,
					Path:     "/",
					Secure:   true,
					SameSite: common.CookieSameSiteLax,
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
			addCookies: []*common.Cookie{
				{
					Name:     "fooCookie",
					Value:    "fooValue",
					URL:      "https://foo.com",
					SameSite: common.CookieSameSiteNone,
				},
				{
					Name:     "barCookie",
					Value:    "barValue",
					URL:      "https://bar.com",
					SameSite: common.CookieSameSiteLax,
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
			addCookies: []*common.Cookie{
				{
					Name:     "fooCookie",
					Value:    "fooValue",
					URL:      "https://foo.com",
					SameSite: common.CookieSameSiteNone,
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
			assert.Equalf(t,
				tt.wantContextCookies, cookies,
				"incorrect cookies received from the browser context",
			)
		})
	}
}

func TestBrowserContextClearCookies(t *testing.T) {
	t.Parallel()

	// add a cookie and clear it out

	tb := newTestBrowser(t, withHTTPServer())
	p := tb.NewPage(nil)
	bctx := p.Context()

	err := bctx.AddCookies(
		[]*common.Cookie{
			{
				Name:  "test_cookie_name",
				Value: "test_cookie_value",
				URL:   "http://test.go",
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, bctx.ClearCookies())

	cookies, err := bctx.Cookies()
	require.NoError(t, err)
	require.Emptyf(t, cookies, "want no cookies, but got: %#v", cookies)
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

func TestBrowserContextTimeout(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                     string
		defaultTimeout           time.Duration
		defaultNavigationTimeout time.Duration
	}{
		{
			name:           "fail when timeout exceeds default timeout",
			defaultTimeout: 1 * time.Millisecond,
		},
		{
			name:                     "fail when timeout exceeds default navigation timeout",
			defaultNavigationTimeout: 1 * time.Millisecond,
		},
		{
			name:                     "default navigation timeout supersedes default timeout",
			defaultTimeout:           30 * time.Second,
			defaultNavigationTimeout: 1 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withHTTPServer())

			tb.withHandler("/slow", func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(100 * time.Millisecond)
				fmt.Fprintf(w, `sorry for being so slow`)
			})

			bc, err := tb.NewContext(nil)
			require.NoError(t, err)

			if tc.defaultTimeout != 0 {
				bc.SetDefaultTimeout(tc.defaultTimeout.Milliseconds())
			}
			if tc.defaultNavigationTimeout != 0 {
				bc.SetDefaultNavigationTimeout(tc.defaultNavigationTimeout.Milliseconds())
			}

			p, err := bc.NewPage()
			require.NoError(t, err)

			res, err := p.Goto(tb.url("/slow"), nil)
			require.Nil(t, res)
			assert.ErrorContains(t, err, "timed out after")
		})
	}
}

func TestBrowserContextWaitForEvent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		event           string
		optsOrPredicate *optsOrPredicate
		wantErr         string
	}{
		{
			// No predicate or options.
			name:  "success",
			event: "page",
		},
		{
			// With a predicate function but not options.
			name:            "success_with_predicate",
			event:           "page",
			optsOrPredicate: &optsOrPredicate{justPredicate: stringPtr("() => true;")},
		},
		{
			// With a predicate function in an option object.
			name:            "success_with_option_predicate",
			event:           "page",
			optsOrPredicate: &optsOrPredicate{predicate: stringPtr("() => true;")},
		},
		{
			// With a predicate function and a new timeout in an option object.
			name:            "success_with_option_predicate_timeout",
			event:           "page",
			optsOrPredicate: &optsOrPredicate{predicate: stringPtr("() => true;"), timeout: int64Ptr(1000)},
		},
		{
			// Fails when an event other than "page" is passed in.
			name:    "fails_incorrect_event",
			event:   "browser",
			wantErr: `incorrect event "browser", "page" is the only event supported`,
		},
		{
			// Fails when the timeout fires while waiting on waitForEvent.
			name:            "fails_timeout",
			event:           "page",
			optsOrPredicate: &optsOrPredicate{predicate: stringPtr("() => false;"), timeout: int64Ptr(10)},
			wantErr:         "waitForEvent timed out after 10ms",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)

			bc, err := tb.NewContext(nil)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(tb.context(), 5*time.Second)
			defer cancel()

			var p1, p2 common.PageAPI
			// We need to run waitForEvent in parallel to the page creation.
			// If we run them synchronously then waitForEvent will wait
			// indefinitely and eventually the test will timeout.
			err = tb.run(
				ctx,
				func() error {
					op := optsOrPredicateToGojaValue(t, tb, tc.optsOrPredicate)
					resp, err := bc.WaitForEvent(tc.event, op)
					if resp != nil {
						var ok bool
						p1, ok = resp.(common.PageAPI)
						require.True(t, ok)
					}
					return err
				},
				func() error {
					var err error
					p2, err = bc.NewPage()
					return err
				},
			)

			if tc.wantErr == "" {
				assert.NoError(t, err)
				// We want to make sure that the page that was created with
				// newPage matches the return value from waitForEvent.
				assert.Equal(t, p1.MainFrame().ID(), p2.MainFrame().ID())
				return
			}

			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// optsOrPredicate is a helper type to enable us to package up the optional
// arguments which could either be a predicate function, or the options object
// which contains a predicate function and a timeout.
type optsOrPredicate struct {
	predicate     *string
	timeout       *int64
	justPredicate *string
}

func stringPtr(value string) *string {
	strPointer := new(string)
	*strPointer = value
	return strPointer
}

func int64Ptr(value int64) *int64 {
	int64Pointer := new(int64)
	*int64Pointer = value
	return int64Pointer
}

// optsOrPredicateToGojaValue will take optsOrPredicate and correctly define the
// optional options where necessary. It will either return nil, a predicate
// function or an options object.
func optsOrPredicateToGojaValue(t *testing.T, tb *testBrowser, op *optsOrPredicate) goja.Value {
	t.Helper()

	// Options or predicate are undefined.
	if op == nil {
		return nil
	}

	// The optional argument is a predicate function.
	if op.justPredicate != nil {
		predicate, err := tb.runJavaScript(*op.justPredicate)
		require.NoError(t, err)
		return predicate
	}

	// The option argument is a options object with only the predicate function
	// defined but no timeout.
	if op.predicate != nil && op.timeout == nil {
		predicate, err := tb.runJavaScript(*op.predicate)
		require.NoError(t, err)

		opts := tb.toGojaValue(struct {
			Predicate goja.Value
		}{
			Predicate: predicate,
		})

		return opts.ToObject(tb.runtime())
	}

	// The option argument is a options object with only timeout.
	if op.predicate == nil && op.timeout != nil {
		opts := tb.toGojaValue(struct {
			Timeout int64
		}{
			Timeout: *op.timeout,
		})

		return opts.ToObject(tb.runtime())
	}

	// The option argument is a options object with both a predicate function
	// and a timeout.
	predicate, err := tb.runJavaScript(*op.predicate)
	require.NoError(t, err)

	opts := tb.toGojaValue(struct {
		Predicate goja.Value
		Timeout   int64
	}{
		Predicate: predicate,
		Timeout:   *op.timeout,
	})

	return opts.ToObject(tb.runtime())
}
