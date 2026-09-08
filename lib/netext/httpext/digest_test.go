package httpext

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mccutchen/go-httpbin/v2/httpbin"
	"github.com/stretchr/testify/require"
)

// A 401 response whose WWW-Authenticate header offers no digest authentication
// (e.g. "Negotiate, NTLM", as IIS commonly sends) must be returned to the caller
// with a readable body, instead of the response body that digest.Transport
// already drained and closed, or the process panic the previous digest library
// caused (index out of range [1] with length 1).
func TestDigestTransportNonDigestChallenge(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", "Negotiate, NTLM")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("no digest here"))
	}))
	t.Cleanup(srv.Close)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	rt := newDigestTransport(srv.Client().Transport, "user", "pass", nil)

	res, err := rt.RoundTrip(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Equal(t, "Negotiate, NTLM", res.Header.Get("WWW-Authenticate"))

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err, "body must be readable even without a digest challenge")
	require.Empty(t, body)
	require.NoError(t, res.Body.Close())
}

// A malformed digest challenge must not fail the whole request either: the
// original 401 response is still returned, and only a warning is logged.
func TestDigestTransportMalformedChallenge(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Digest realm="unclosed, qop="auth"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	rt := newDigestTransport(srv.Client().Transport, "user", "pass", nil)

	res, err := rt.RoundTrip(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.Contains(t, res.Header.Get("WWW-Authenticate"), "Digest")
}

// The regular digest handshake keeps working end-to-end, including a request
// that succeeds on the first attempt because the server requires no
// authentication.
func TestDigestTransportSuccess(t *testing.T) {
	t.Parallel()
	hb := httpbin.New()
	srv := httptest.NewServer(hb.Handler())
	t.Cleanup(srv.Close)

	newReq := func(url string) *http.Request {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
		require.NoError(t, err)
		return req
	}

	t.Run("digest authentication", func(t *testing.T) {
		t.Parallel()
		rt := newDigestTransport(srv.Client().Transport, "testuser", "testpwd", nil)
		res, err := rt.RoundTrip(newReq(srv.URL + "/digest-auth/auth/testuser/testpwd"))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("digest authentication with SHA-256", func(t *testing.T) {
		t.Parallel()
		rt := newDigestTransport(srv.Client().Transport, "testuser", "testpwd", nil)
		res, err := rt.RoundTrip(newReq(srv.URL + "/digest-auth/auth/testuser/testpwd/SHA-256"))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("no authentication needed", func(t *testing.T) {
		t.Parallel()
		rt := newDigestTransport(srv.Client().Transport, "testuser", "testpwd", nil)
		res, err := rt.RoundTrip(newReq(srv.URL + "/get"))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
	})
}
