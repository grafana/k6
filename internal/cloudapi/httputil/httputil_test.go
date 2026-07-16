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

type closeRecorder struct {
	io.Reader
	closeErr error
	closed   bool
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return c.closeErr
}

func TestCloseResponse(t *testing.T) {
	t.Parallel()

	t.Run("nil response does not panic", func(t *testing.T) {
		t.Parallel()
		var rerr error
		assert.NotPanics(t, func() { CloseResponse(nil, &rerr) })
	})

	t.Run("close error is assigned when rerr is nil", func(t *testing.T) {
		t.Parallel()
		closeErr := errors.New("close failed")
		body := &closeRecorder{Reader: io.LimitReader(errReader{}, 0), closeErr: closeErr}
		res := &http.Response{Body: body}

		var rerr error
		CloseResponse(res, &rerr)

		assert.Equal(t, closeErr, rerr)
		assert.True(t, body.closed)
	})

	t.Run("close error does not overwrite an existing error", func(t *testing.T) {
		t.Parallel()
		closeErr := errors.New("close failed")
		existingErr := errors.New("already set")
		body := &closeRecorder{Reader: io.LimitReader(errReader{}, 0), closeErr: closeErr}
		res := &http.Response{Body: body}

		rerr := existingErr
		CloseResponse(res, &rerr)

		require.Error(t, rerr)
		assert.Equal(t, existingErr, rerr)
	})
}

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
		{name: "in range", in: 42, want: 42},
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
