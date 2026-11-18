package common

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common/js"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
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

func TestBrowserContextCleanupClosesClientCertificateProxy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		downloadsPath func(t *testing.T) string
		wantErr       string
	}{
		{
			name:          "cleanup_ok",
			downloadsPath: func(t *testing.T) string { return filepath.Join(t.TempDir(), "downloads") },
		},
		{
			name:          "cleanup_fails",
			downloadsPath: func(*testing.T) string { return "invalid\x00path" },
			wantErr:       "removing downloads path",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := clientCertificateTestOptions(t)
			opts.DownloadsPath = tc.downloadsPath(t)
			bc, err := newClientCertificateTestBrowser(t, nil).NewContext(opts)
			require.NoError(t, err)
			addr := bc.clientCertificateProxy.Addr()
			requireAcceptsConnections(t, addr)

			err = bc.cleanup()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
			assertRefusesConnections(t, addr)
		})
	}
}

func TestBrowserContextCloseClosesClientCertificateProxy(t *testing.T) {
	t.Parallel()

	disposeErr := errors.New("dispose failed")
	bc, err := newClientCertificateTestBrowser(t, disposeErr).NewContext(clientCertificateTestOptions(t))
	require.NoError(t, err)
	addr := bc.clientCertificateProxy.Addr()
	requireAcceptsConnections(t, addr)

	assert.ErrorIs(t, bc.Close(), disposeErr)
	assertRefusesConnections(t, addr)
}

func TestBrowserContextClientCertificateCredentials(t *testing.T) {
	t.Parallel()

	opts := clientCertificateTestOptions(t)
	opts.HTTPCredentials = Credentials{Username: "user-0", Password: "pass-0"}
	bc, err := newClientCertificateTestBrowser(t, nil).NewContext(opts)
	require.NoError(t, err)
	require.NotNil(t, bc.proxyCredentials)
	get := bc.proxyCredentials.get

	username, password := get()
	assert.Equal(t, "user-0", username)
	assert.Equal(t, "pass-0", password)

	const updates = 100
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 1; i <= updates; i++ {
			assert.NoError(t, bc.SetHTTPCredentials(Credentials{
				Username: fmt.Sprintf("user-%d", i), Password: fmt.Sprintf("pass-%d", i),
			}))
		}
	})
	for range 4 {
		wg.Go(func() {
			for range updates {
				username, password := get()
				assert.Equal(t, "pass"+username[len("user"):], password, "torn credentials")
			}
		})
	}
	wg.Wait()

	username, password = get()
	assert.Equal(t, fmt.Sprintf("user-%d", updates), username)
	assert.Equal(t, fmt.Sprintf("pass-%d", updates), password)
}
