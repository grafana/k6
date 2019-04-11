package httpext

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type reader func([]byte) (int, error)

func (r reader) Read(a []byte) (int, error) {
	return ((func([]byte) (int, error))(r))(a)
}

const badReadMsg = "bad read error for test"
const badCloseMsg = "bad close error for test"

func badReadBody() io.Reader {
	return reader(func(_ []byte) (int, error) {
		return 0, errors.New(badReadMsg)
	})
}

type closer func() error

func (c closer) Close() error {
	return ((func() error)(c))()
}

func badCloseBody() io.ReadCloser {
	return struct {
		io.Reader
		io.Closer
	}{
		Reader: reader(func(_ []byte) (int, error) {
			return 0, io.EOF
		}),
		Closer: closer(func() error {
			return errors.New(badCloseMsg)
		}),
	}
}

func TestMakeRequestError(t *testing.T) {
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	t.Run("bad read body", func(t *testing.T) {
		req, err := http.NewRequest("GET", "https://wont.be.used", badReadBody())
		require.NoError(t, err)
		var preq = &ParsedHTTPRequest{
			Req:          req,
			Compressions: []CompressionType{CompressionTypeGzip},
		}
		_, err = MakeRequest(ctx, preq)
		require.Error(t, err)
		require.Equal(t, err.Error(), badReadMsg)
	})

	t.Run("bad close body", func(t *testing.T) {
		req, err := http.NewRequest("GET", "https://wont.be.used", badCloseBody())
		require.NoError(t, err)
		var preq = &ParsedHTTPRequest{
			Req:          req,
			Compressions: []CompressionType{CompressionTypeGzip},
		}
		_, err = MakeRequest(ctx, preq)
		require.Error(t, err)
		require.Equal(t, err.Error(), badCloseMsg)
	})

	t.Run("bad compression algorithm body", func(t *testing.T) {
		req, err := http.NewRequest("GET", "https://wont.be.used", new(bytes.Buffer))
		require.NoError(t, err)
		var preq = &ParsedHTTPRequest{
			Req:          req,
			Compressions: []CompressionType{CompressionType(13)},
		}
		_, err = MakeRequest(ctx, preq)
		require.Error(t, err)
		require.Equal(t, err.Error(), "unknown compressionType CompressionType(13)")
	})
}
