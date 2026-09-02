package httpext

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingReadCloser struct {
	err    error
	closed bool
}

func (b *failingReadCloser) Read(_ []byte) (int, error) {
	return 0, b.err
}

func (b *failingReadCloser) Close() error {
	b.closed = true
	return nil
}

type staticResponseTransport struct {
	response *http.Response
}

func (t staticResponseTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return t.response, nil
}

func TestDigestTransportClosesUnauthorizedResponseBodyOnReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read failed")
	body := &failingReadCloser{err: readErr}
	transport := digestTransport{
		originalTransport: staticResponseTransport{
			response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       body,
			},
		},
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://user:password@example.com", nil)
	require.NoError(t, err)

	// The response must be nil when reading the challenge body fails.
	response, err := transport.RoundTrip(req) //nolint:bodyclose

	require.ErrorIs(t, err, readErr)
	assert.Nil(t, response)
	assert.True(t, body.closed)
}
