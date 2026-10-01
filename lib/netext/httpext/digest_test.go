package httpext

import (
	"errors"
	"net/http"
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
		"user", "pass", nil,
	)

	res, err := rt.RoundTrip(req)
	t.Cleanup(func() { _ = res.Body.Close() })

	// The challenge-less 401 is surfaced (not the read error), and the failed
	// body was still closed underneath.
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	require.True(t, body.closed, "challenge response body must be closed on read errors")
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
