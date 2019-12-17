package httpext

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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

	t.Run("invalid upgrade response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Connection", "Upgrade")
			w.Header().Add("Upgrade", "h2c")
			w.WriteHeader(http.StatusSwitchingProtocols)
		}))
		defer srv.Close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		logger := logrus.New()
		logger.Level = logrus.DebugLevel
		state := &lib.State{
			Options:   lib.Options{RunTags: &stats.SampleTags{}},
			Transport: srv.Client().Transport,
			Logger:    logger,
		}
		ctx = lib.WithState(ctx, state)
		req, _ := http.NewRequest("GET", srv.URL, nil)
		var preq = &ParsedHTTPRequest{
			Req:     req,
			URL:     &URL{u: req.URL},
			Body:    new(bytes.Buffer),
			Timeout: 10 * time.Second,
		}

		res, err := MakeRequest(ctx, preq)

		assert.Nil(t, res)
		assert.EqualError(t, err, "unsupported response status: 101 Switching Protocols")
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

func TestMakeRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	samples := make(chan stats.SampleContainer, 10)
	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	state := &lib.State{
		Options:   lib.Options{RunTags: &stats.SampleTags{}},
		Transport: srv.Client().Transport,
		Samples:   samples,
		Logger:    logger,
	}
	ctx = lib.WithState(ctx, state)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	var preq = &ParsedHTTPRequest{
		Req:     req,
		URL:     &URL{u: req.URL},
		Body:    new(bytes.Buffer),
		Timeout: 10 * time.Millisecond,
	}

	res, err := MakeRequest(ctx, preq)
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, samples, 1)
}

func BenchmarkWrapDecompressionError(b *testing.B) {
	err := errors.New("error")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = wrapDecompressionError(err)
	}
}
