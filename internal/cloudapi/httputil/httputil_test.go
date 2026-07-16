package httputil

import (
	"bytes"
	"errors"
	"io"
	"math"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closeRecorder is an io.ReadCloser that records whether it was closed and,
// optionally, reads from an underlying reader before returning a
// configurable error on Close.
type closeRecorder struct {
	io.Reader
	closeErr error
	closed   bool
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return c.closeErr
}

func TestCloseResponseNilResponse(t *testing.T) {
	t.Parallel()

	var rerr error
	assert.NotPanics(t, func() {
		CloseResponse(nil, &rerr)
	})
	assert.NoError(t, rerr)
}

func TestCloseResponseSuccess(t *testing.T) {
	t.Parallel()

	body := &closeRecorder{Reader: io.LimitReader(errReader{}, 0)}
	res := &http.Response{Body: body}

	var rerr error
	CloseResponse(res, &rerr)

	assert.True(t, body.closed)
	assert.NoError(t, rerr)
}

func TestCloseResponseSetsErrorWhenNilRerr(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("close failed")
	body := &closeRecorder{Reader: io.LimitReader(errReader{}, 0), closeErr: closeErr}
	res := &http.Response{Body: body}

	var rerr error
	CloseResponse(res, &rerr)

	require.Error(t, rerr)
	assert.Equal(t, closeErr, rerr)
	assert.True(t, body.closed)
}

func TestCloseResponseDoesNotOverwriteExistingError(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("close failed")
	existingErr := errors.New("already set")
	body := &closeRecorder{Reader: io.LimitReader(errReader{}, 0), closeErr: closeErr}
	res := &http.Response{Body: body}

	rerr := existingErr
	CloseResponse(res, &rerr)

	require.Error(t, rerr)
	assert.Equal(t, existingErr, rerr)
	assert.True(t, body.closed)
}

// errReader is an io.Reader that always returns io.EOF without producing data.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.EOF }

// recordingRoundTripper records every request it sees and returns a fixed
// response.
type recordingRoundTripper struct {
	bodies [][]byte
	resp   *http.Response
	err    error
}

func (rt *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var b []byte
	if req.Body != nil {
		b, _ = io.ReadAll(req.Body)
	}
	rt.bodies = append(rt.bodies, b)
	return rt.resp, rt.err
}

func TestBodyResetTransportResetsBodyWhenGetBodySet(t *testing.T) {
	t.Parallel()

	want := []byte("hello")
	rt := &recordingRoundTripper{resp: &http.Response{StatusCode: http.StatusOK}}
	transport := BodyResetTransport{Base: rt}

	req, err := http.NewRequest(http.MethodPost, "http://example.com", nil) //nolint:noctx
	require.NoError(t, err)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(want)), nil
	}

	_, err = transport.RoundTrip(req) //nolint:bodyclose // canned response has no real body
	require.NoError(t, err)
	_, err = transport.RoundTrip(req) //nolint:bodyclose // canned response has no real body
	require.NoError(t, err)

	require.Len(t, rt.bodies, 2)
	assert.Equal(t, want, rt.bodies[0])
	assert.Equal(t, want, rt.bodies[1])
}

func TestBodyResetTransportPassesThroughWhenNoGetBody(t *testing.T) {
	t.Parallel()

	rt := &recordingRoundTripper{resp: &http.Response{StatusCode: http.StatusOK}}
	transport := BodyResetTransport{Base: rt}

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil) //nolint:noctx
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req) //nolint:bodyclose // canned response has no real body
	require.NoError(t, err)
	assert.Same(t, rt.resp, resp)
}

func TestToInt32(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      int64
		want    int32
		wantErr bool
	}{
		{name: "zero", in: 0, want: 0},
		{name: "positive in range", in: 42, want: 42},
		{name: "max int32", in: math.MaxInt32, want: math.MaxInt32},
		{name: "min int32", in: math.MinInt32, want: math.MinInt32},
		{name: "overflow above", in: math.MaxInt32 + 1, wantErr: true},
		{name: "overflow below", in: math.MinInt32 - 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ToInt32(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
