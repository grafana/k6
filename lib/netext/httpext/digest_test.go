package httpext

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// The body of a 401 challenge response must be closed even when reading it
// fails, carrying over the guarantee added for the previous transport
// implementation in #6355: the wrapped digest.Transport drains and closes the
// challenge body unconditionally before parsing it.
//
// The rest of the digest behavior is covered end-to-end by the higher-level
// tests in js/modules/k6/http/request_test.go.
func TestDigestTransportClosesChallengeBodyOnReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read failed")
	body := &failingReadCloser{err: readErr}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://user:password@example.com", nil)
	require.NoError(t, err)
	rt := newDigestTransport(
		staticResponseTransport{response: &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{"WWW-Authenticate": []string{"Negotiate, NTLM"}},
			Body:       body,
			Request:    req,
		}},
		"user", "pass", req.URL, nil,
	)

	res, err := rt.RoundTrip(req)
	t.Cleanup(func() { _ = res.Body.Close() })

	// The challenge-less 401 is surfaced (not the read error), and the failed
	// body was still closed underneath.
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.True(t, body.closed, "challenge response body must be closed on read errors")
}

// Credentials configured for one host must not be used when a redirect lands on
// another host. Otherwise an open redirect in front of a digest-protected URL
// can collect a Digest Authorization header (username plus a password-derived
// response) for an attacker-chosen challenge.
func TestDigestTransportDoesNotForwardCredentialsAcrossHosts(t *testing.T) {
	t.Parallel()

	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			leaked.Store(true)
		}
		w.Header().Set("WWW-Authenticate", `Digest realm="evil", nonce="n", qop="auth", algorithm=MD5`)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "nope")
	}))
	t.Cleanup(other.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	originURL, err := url.Parse(origin.URL)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, origin.URL+"/start", nil)
	require.NoError(t, err)
	client := http.Client{
		Transport: newDigestTransport(http.DefaultTransport, "user", "password", originURL, nil),
	}
	res, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = res.Body.Close() })

	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.False(t, leaked.Load(), "digest credentials were sent to the redirect target")
}

// A same-origin redirect must still perform the digest handshake. Scoping
// credentials to the original origin must not disable authentication for a
// later hop on that same origin.
func TestDigestTransportAuthenticatesSameOriginRedirect(t *testing.T) {
	t.Parallel()

	var sawDigest atomic.Bool
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, srv.URL+"/secret", http.StatusFound)
			return
		}
		if strings.HasPrefix(r.Header.Get("Authorization"), "Digest ") {
			sawDigest.Store(true)
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
			return
		}
		w.Header().Set("WWW-Authenticate", `Digest realm="x", nonce="abc", qop="auth", algorithm=MD5`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	originURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/start", nil)
	require.NoError(t, err)
	client := http.Client{
		Transport: newDigestTransport(http.DefaultTransport, "user", "password", originURL, nil),
	}
	res, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = res.Body.Close() })

	require.Equal(t, http.StatusOK, res.StatusCode)
	require.True(t, sawDigest.Load(), "same-origin redirect should still send digest credentials")
}

func TestCanonicalAuthorityScopesDigestCredentials(t *testing.T) {
	t.Parallel()

	parse := func(raw string) *url.URL {
		t.Helper()
		u, err := url.Parse(raw)
		require.NoError(t, err)
		return u
	}

	base := canonicalAuthority(parse("http://Example.COM/a"))
	require.Equal(t, base, canonicalAuthority(parse("http://example.com:80/b")))
	require.Equal(t, "https://example.com:443", canonicalAuthority(parse("https://example.com/a")))
	require.NotEqual(t, base, canonicalAuthority(parse("https://example.com/a")))
	require.NotEqual(t, base, canonicalAuthority(parse("http://example.com:81/a")))
	require.NotEqual(t, base, canonicalAuthority(parse("http://other.example/a")))
	require.Equal(t, "http://[::1]:80", canonicalAuthority(parse("http://[::1]/a")))
}

type failingReadCloser struct {
	err    error
	closed bool
}

func (b *failingReadCloser) Read(_ []byte) (int, error) { return 0, b.err }

func (b *failingReadCloser) Close() error {
	b.closed = true
	return nil
}

type staticResponseTransport struct {
	response *http.Response
}

func (t staticResponseTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return t.response, nil
}
