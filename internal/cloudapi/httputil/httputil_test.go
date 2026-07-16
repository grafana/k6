package httputil

import (
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
