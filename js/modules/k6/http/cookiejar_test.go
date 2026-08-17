package http

import (
	"net/http/cookiejar"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCookieJarCookiesForURLInvalidURL(t *testing.T) {
	t.Parallel()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	cj := CookieJar{Jar: jar}

	// Unencoded '%' in a query string is a common real-world trigger
	// (e.g. "50% off" interpolated into a URL without encodeURIComponent).
	invalidURLs := []string{
		"%",
		"%zz",
		"https://example.com/search?q=50%",
		"https://example.com/%zz",
		"http://[::1",
	}

	for _, rawURL := range invalidURLs {
		t.Run(rawURL, func(t *testing.T) {
			t.Parallel()
			assert.NotPanics(t, func() {
				_, err := cj.CookiesForURL(rawURL)
				assert.Error(t, err)
			})
		})
	}
}

func TestCookieJarCookiesForURLValidURL(t *testing.T) {
	t.Parallel()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	cj := CookieJar{Jar: jar}

	cookies, err := cj.CookiesForURL("https://example.com/path")
	require.NoError(t, err)
	assert.Empty(t, cookies)
}

func TestCookieJarCookiesForURLJSThrowsInsteadOfPanicking(t *testing.T) {
	t.Parallel()

	runtime, _ := getTestModuleInstance(t)
	rt := runtime.VU.RuntimeField

	_, err := rt.RunString(`
		const jar = new http.CookieJar();
		let threw = false;
		try {
			jar.cookiesForURL("https://shop.example.com/search?q=50%");
		} catch (e) {
			threw = true;
			if (!String(e).includes("invalid URL escape")) {
				throw new Error("unexpected error: " + e);
			}
		}
		if (!threw) {
			throw new Error("cookiesForURL should throw on an invalid URL instead of succeeding");
		}

		// Valid URLs, including correctly encoded percent signs, still work.
		const ok = jar.cookiesForURL("https://shop.example.com/search?q=50%25");
		if (ok === undefined || ok === null) {
			throw new Error("cookiesForURL should return a cookie map for a valid URL");
		}
	`)
	require.NoError(t, err)
}
