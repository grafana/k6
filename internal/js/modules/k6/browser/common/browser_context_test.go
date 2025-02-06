package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common/js"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

func TestNewBrowserContext(t *testing.T) {
	t.Parallel()

	t.Run("add_web_vital_js_scripts_to_context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		logger := log.NewNullLogger()
		b := newBrowser(context.Background(), ctx, cancel, nil, NewLocalBrowserOptions(), logger)

		vu := k6test.NewVU(t)
		ctx = k6ext.WithVU(ctx, vu)

		bc, err := NewBrowserContext(ctx, b, "some-id", nil, nil)
		require.NoError(t, err)

		webVitalIIFEScriptFound := false
		webVitalInitScriptFound := false
		for _, script := range bc.evaluateOnNewDocumentSources {
			switch script {
			case js.WebVitalIIFEScript:
				webVitalIIFEScriptFound = true
			case js.WebVitalInitScript:
				webVitalInitScriptFound = true
			default:
				assert.Fail(t, "script is neither WebVitalIIFEScript, nor WebVitalInitScript")
			}
		}

		assert.True(t, webVitalIIFEScriptFound, "WebVitalIIFEScript was not initialized in the context")
		assert.True(t, webVitalInitScriptFound, "WebVitalInitScript was not initialized in the context")
	})
}

func TestSetDownloadsPath(t *testing.T) {
	t.Parallel()

	t.Run("empty_path", func(t *testing.T) {
		t.Parallel()

		var bc BrowserContext
		require.NoError(t, bc.setDownloadsPath(""))
		assert.NotEmpty(t, bc.DownloadsPath)
		assert.Contains(t, bc.DownloadsPath, artifactsDirectory)
		assert.DirExists(t, bc.DownloadsPath)
	})
	t.Run("non_empty_path", func(t *testing.T) {
		t.Parallel()

		var bc BrowserContext
		path := "/my/directory"
		require.NoError(t, bc.setDownloadsPath(path))
		assert.Equal(t, path, bc.DownloadsPath)
	})
	t.Run("cleanup", func(t *testing.T) {
		t.Parallel()

		var bc BrowserContext
		require.NoError(t, bc.setDownloadsPath(""))
		assert.DirExists(t, bc.DownloadsPath)
		require.NoError(t, bc.cleanup())
		assert.NoDirExists(t, bc.DownloadsPath)
	})
}

func TestFilterCookies(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		filterByURLs []string
		cookies      []*Cookie
		wantCookies  []*Cookie
		wantErr      bool
	}{
		"no_cookies": {
			filterByURLs: []string{"https://example.com"},
			cookies:      nil,
			wantCookies:  nil,
		},
		"filter_none": {
			filterByURLs: nil,
			cookies: []*Cookie{
				{
					Domain: "foo.com",
				},
				{
					Domain: "bar.com",
				},
			},
			wantCookies: []*Cookie{
				{
					Domain: "foo.com",
				},
				{
					Domain: "bar.com",
				},
			},
		},
		"filter_by_url": {
			filterByURLs: []string{
				"https://foo.com",
			},
			cookies: []*Cookie{
				{
					Domain: "foo.com",
				},
				{
					Domain: "bar.com",
				},
				{
					Domain: "baz.com",
				},
			},
			wantCookies: []*Cookie{
				{
					Domain: "foo.com",
				},
			},
		},
		"filter_by_urls": {
			filterByURLs: []string{
				"https://foo.com",
				"https://baz.com",
			},
			cookies: []*Cookie{
				{
					Domain: "foo.com",
				},
				{
					Domain: "bar.com",
				},
				{
					Domain: "baz.com",
				},
			},
			wantCookies: []*Cookie{
				{
					Domain: "foo.com",
				},
				{
					Domain: "baz.com",
				},
			},
		},
		"filter_by_subdomain": {
			filterByURLs: []string{
				"https://sub.foo.com",
			},
			cookies: []*Cookie{
				{
					Domain: "sub.foo.com",
				},
				{
					Domain: ".foo.com",
				},
			},
			wantCookies: []*Cookie{
				{
					Domain: "sub.foo.com",
				},
			},
		},
		"filter_dot_prefixed_domains": {
			filterByURLs: []string{
				"https://foo.com",
			},
			cookies: []*Cookie{
				{
					Domain: ".foo.com",
				},
			},
			wantCookies: []*Cookie{
				{
					Domain: ".foo.com",
				},
			},
		},
		"filter_by_secure_cookies": {
			filterByURLs: []string{
				"https://foo.com",
			},
			cookies: []*Cookie{
				{
					Domain: "foo.com",
					Secure: true,
				},
			},
			wantCookies: []*Cookie{
				{
					Domain: "foo.com",
					Secure: true,
				},
			},
		},
		"filter_by_http_only_cookies": {
			filterByURLs: []string{
				"https://foo.com",
			},
			cookies: []*Cookie{
				{
					Domain:   "foo.com",
					HTTPOnly: true,
				},
			},
			wantCookies: []*Cookie{
				{
					Domain:   "foo.com",
					HTTPOnly: true,
				},
			},
		},
		"filter_by_path": {
			filterByURLs: []string{
				"https://foo.com/bar",
			},
			cookies: []*Cookie{
				{
					Domain: "foo.com",
					Path:   "/bar",
				},
				{
					Domain: "foo.com",
					Path:   "/baz",
				},
			},
			wantCookies: []*Cookie{
				{
					Domain: "foo.com",
					Path:   "/bar",
				},
			},
		},
		"allow_secure_cookie_on_localhost": {
			filterByURLs: []string{
				"http://localhost",
			},
			cookies: []*Cookie{
				{
					Domain: "localhost",
					Secure: true,
				},
			},
			wantCookies: []*Cookie{
				{
					Domain: "localhost",
					Secure: true,
				},
			},
		},
		"disallow_secure_cookie_on_http": {
			filterByURLs: []string{
				"http://foo.com",
			},
			cookies: []*Cookie{
				{
					Domain: "foo.com",
					Secure: true,
				},
			},
			wantCookies: nil,
		},
		"invalid_filter": {
			filterByURLs: []string{
				"HELLO WORLD!",
			},
			cookies: []*Cookie{
				{
					Domain: "foo.com",
				},
			},
			wantCookies: nil,
			wantErr:     true,
		},
		"invalid_filter_empty": {
			filterByURLs: []string{
				"",
			},
			cookies: []*Cookie{
				{
					Domain: "foo.com",
				},
			},
			wantCookies: nil,
			wantErr:     true,
		},
		"invalid_filter_multi": {
			filterByURLs: []string{
				"https://foo.com", "", "HELLO WORLD",
			},
			cookies: []*Cookie{
				{
					Domain: "foo.com",
				},
			},
			wantCookies: nil,
			wantErr:     true,
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cookies, err := filterCookies(
				tt.cookies,
				tt.filterByURLs...,
			)
			if tt.wantErr {
				assert.Nilf(t, cookies, "want no cookies after an error, but got %#v", cookies)
				require.Errorf(t, err, "want an error, but got none")
				return
			}
			require.NoError(t, err)

			assert.Equalf(t,
				tt.wantCookies, cookies,
				"incorrect cookies filtered by the filter %#v", tt.filterByURLs,
			)
		})
	}
}
