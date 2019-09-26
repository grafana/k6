package httpext

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
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

func TestCompressionBodyError(t *testing.T) {
	var algos = []CompressionType{CompressionTypeGzip}
	t.Run("bad read body", func(t *testing.T) {
		_, _, err := compressBody(algos, ioutil.NopCloser(badReadBody()))
		require.Error(t, err)
		require.Equal(t, err.Error(), badReadMsg)
	})

	t.Run("bad close body", func(t *testing.T) {
		_, _, err := compressBody(algos, badCloseBody())
		require.Error(t, err)
		require.Equal(t, err.Error(), badCloseMsg)
	})
}

func TestMakeRequestError(t *testing.T) {
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	t.Run("bad compression algorithm body", func(t *testing.T) {
		var req, err = http.NewRequest("GET", "https://wont.be.used", nil)

		require.NoError(t, err)
		var badCompressionType = CompressionType(13)
		require.False(t, badCompressionType.IsACompressionType())
		var preq = &ParsedHTTPRequest{
			Req:          req,
			Body:         new(bytes.Buffer),
			Compressions: []CompressionType{badCompressionType},
		}
		_, err = MakeRequest(ctx, preq)
		require.Error(t, err)
		require.Equal(t, err.Error(), "unknown compressionType CompressionType(13)")
	})
}

func TestURL(t *testing.T) {
	t.Run("Clean", func(t *testing.T) {
		testCases := []struct {
			url      string
			expected string
		}{
			{"https://example.com/", "https://example.com/"},
			{"https://example.com/${}", "https://example.com/${}"},
			{"https://user@example.com/", "https://****@example.com/"},
			{"https://user:pass@example.com/", "https://****:****@example.com/"},
			{"https://user:pass@example.com/path?a=1&b=2", "https://****:****@example.com/path?a=1&b=2"},
			{"https://user:pass@example.com/${}/${}", "https://****:****@example.com/${}/${}"},
			{"@malformed/url", "@malformed/url"},
			{"not a url", "not a url"},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.url, func(t *testing.T) {
				u, err := url.Parse(tc.url)
				require.NoError(t, err)
				ut := URL{u: u, URL: tc.url}
				require.Equal(t, tc.expected, ut.Clean())
			})
		}
	})
}
