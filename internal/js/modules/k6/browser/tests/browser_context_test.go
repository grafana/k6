package tests

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/env"
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
			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.url("/empty"),
				opts,
			)
			require.NoErrorf(t,
				err, "failed to open an empty page",
			)

			// setting document.cookie into the page
			cookie, err := p.Evaluate(tt.documentCookiesSnippet)
			require.NoError(t, err)
			require.Equalf(t,
				tt.wantDocumentCookies,
				cookie,
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

	tests := []struct {
		name      string
		testRunID string
		want      string
	}{
		{
			name: "empty_testRunId",
			want: `{"testRunId":""}`,
		},
		{
			name:      "with_testRunId",
			testRunID: "123456",
			want:      `{"testRunId":"123456"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu, _, _, cleanUp := startIteration(t, env.ConstLookup(env.K6TestRunID, tt.testRunID))
			defer cleanUp()

			// First test with browser.newPage
			got := vu.RunPromise(t, `
				const p = await browser.newPage();
				await p.goto("about:blank");
				const o = await p.evaluate(() => window.k6);
				return JSON.stringify(o);
			`)
			assert.Equal(t, tt.want, got.Result().String())

			// Now test with browser.newContext
			got = vu.RunPromise(t, `
				await browser.closeContext();
				const c = await browser.newContext();
				const p2 = await c.newPage();
				await p2.goto("about:blank");
				const o2 = await p2.evaluate(() => window.k6);
				return JSON.stringify(o2);
			`)
			assert.Equal(t, tt.want, got.Result().String())
		})
	}
}

// This test ensures that when opening a new tab, this it is possible to navigate
// to the url. If the mapping layer is not setup correctly we can end up with a
// NPD.
func TestNewTab(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())

	// Start the iteration
	vu, _, _, cleanUp := startIteration(t, env.ConstLookup(env.K6TestRunID, "12345"))
	defer cleanUp()

	// Run the test script
	_, err := vu.RunAsync(t, `
		const p = await browser.newPage()
		await p.goto("%s")

		const p2 = await browser.context().newPage()
		await p2.goto("%s")
	`, tb.staticURL("ping.html"), tb.staticURL("ping.html"))
	require.NoError(t, err)
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
				_, err := fmt.Fprintf(w, `sorry for being so slow`)
				require.NoError(t, err)
			})

			bc, err := tb.NewContext(nil)
			require.NoError(t, err)

			p, err := bc.NewPage()
			require.NoError(t, err)

			var timeout time.Duration
			if tc.defaultTimeout != 0 {
				timeout = tc.defaultTimeout
				bc.SetDefaultTimeout(tc.defaultTimeout.Milliseconds())
			}
			if tc.defaultNavigationTimeout != 0 {
				timeout = tc.defaultNavigationTimeout
				bc.SetDefaultNavigationTimeout(tc.defaultNavigationTimeout.Milliseconds())
			}
			res, err := p.Goto(
				tb.url("/slow"),
				&common.FrameGotoOptions{
					Timeout: timeout,
				},
			)
			require.Nil(t, res)
			assert.ErrorContains(t, err, "timed out after")
		})
	}
}

func TestBrowserContextWaitForEvent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		event     string
		predicate func(p *common.Page) (bool, error)
		timeout   time.Duration
		wantErr   string
	}{
		{
			// No predicate and default timeout.
			name:    "success",
			event:   "page",
			timeout: 30 * time.Second,
		},
		{
			// With a predicate function and default timeout.
			name:      "success_with_predicate",
			event:     "page",
			predicate: func(p *common.Page) (bool, error) { return true, nil },
			timeout:   30 * time.Second,
		},
		{
			// Fails when an event other than "page" is passed in.
			name:    "fails_incorrect_event",
			event:   "browser",
			wantErr: `incorrect event "browser", "page" is the only event supported`,
		},
		{
			// Fails when the timeout fires while waiting on waitForEvent.
			name:      "fails_timeout",
			event:     "page",
			predicate: func(p *common.Page) (bool, error) { return false, nil },
			timeout:   10 * time.Millisecond,
			wantErr:   "waitForEvent timed out after 10ms",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Use withSkipClose() opt as we will close it manually to force the
			// page.TaskQueue closing, which seems to be a requirement otherwise
			// it doesn't complete the test.
			tb := newTestBrowser(t)

			bc, err := tb.NewContext(nil)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(tb.context(), 5*time.Second)
			defer cancel()

			var (
				aboutToCallWait = make(chan bool)
				p1ID, p2ID      string
			)

			err = tb.run(ctx,
				func() error {
					var resp any
					close(aboutToCallWait)
					resp, err := bc.WaitForEvent(tc.event, tc.predicate, tc.timeout)
					if err != nil {
						return err
					}

					p, ok := resp.(*common.Page)
					if !ok {
						return errors.New("response from waitForEvent is not a page")
					}
					p1ID = p.MainFrame().ID()

					return nil
				},
				func() error {
					<-aboutToCallWait

					if tc.wantErr == "" {
						p, err := bc.NewPage()
						require.NoError(t, err)

						p2ID = p.MainFrame().ID()
					}

					return nil
				},
			)

			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
			// We want to make sure that the page that was created with
			// newPage matches the return value from waitForEvent.
			assert.Equal(t, p1ID, p2ID)
		})
	}
}

func TestBrowserContextGrantPermissions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		permission string
		wantErr    string
	}{
		{name: "geolocation", permission: "geolocation"},
		{name: "midi", permission: "midi"},
		{name: "midi-sysex", permission: "midi-sysex"},
		{name: "notifications", permission: "notifications"},
		{name: "camera", permission: "camera"},
		{name: "microphone", permission: "microphone"},
		{name: "background-sync", permission: "background-sync"},
		{name: "ambient-light-sensor", permission: "ambient-light-sensor"},
		{name: "accelerometer", permission: "accelerometer"},
		{name: "gyroscope", permission: "gyroscope"},
		{name: "magnetometer", permission: "magnetometer"},
		{name: "clipboard-read", permission: "clipboard-read"},
		{name: "clipboard-write", permission: "clipboard-write"},
		{name: "payment-handler", permission: "payment-handler"},
		{name: "fake-permission", permission: "fake-permission", wantErr: `"fake-permission" is an invalid permission`},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			bCtx, err := tb.NewContext(nil)
			require.NoError(t, err)

			err = bCtx.GrantPermissions(
				[]string{tc.permission},
				common.GrantPermissionsOptions{},
			)

			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}

			assert.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestBrowserContextClearPermissions(t *testing.T) {
	t.Parallel()

	hasPermission := func(_ *testBrowser, p *common.Page, perm string) bool {
		t.Helper()

		js := fmt.Sprintf(`
			(perm) => navigator.permissions.query(
				{ name: %q }
			).then(result => result.state)
		`, perm)
		v, err := p.Evaluate(js)
		require.NoError(t, err)
		s := asString(t, v)
		return s == "granted"
	}

	t.Run("no_permissions_set", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		bCtx, err := tb.NewContext(nil)
		require.NoError(t, err)
		p, err := bCtx.NewPage()
		require.NoError(t, err)

		require.False(t, hasPermission(tb, p, "geolocation"))

		err = bCtx.ClearPermissions()
		assert.NoError(t, err)
		require.False(t, hasPermission(tb, p, "geolocation"))
	})

	t.Run("permissions_set", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		bCtx, err := tb.NewContext(nil)
		require.NoError(t, err)
		p, err := bCtx.NewPage()
		require.NoError(t, err)

		require.False(t, hasPermission(tb, p, "geolocation"))

		err = bCtx.GrantPermissions(
			[]string{"geolocation"},
			common.GrantPermissionsOptions{},
		)
		require.NoError(t, err)
		require.True(t, hasPermission(tb, p, "geolocation"))

		err = bCtx.ClearPermissions()
		assert.NoError(t, err)
		require.False(t, hasPermission(tb, p, "geolocation"))
	})
}
