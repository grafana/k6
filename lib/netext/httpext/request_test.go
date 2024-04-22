package httpext

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mccutchen/go-httpbin/httpbin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"golang.org/x/time/rate"
	"gopkg.in/guregu/null.v3"
)

type reader func([]byte) (int, error)

func (r reader) Read(a []byte) (int, error) {
	return ((func([]byte) (int, error))(r))(a)
}

const (
	badReadMsg  = "bad read error for test"
	badCloseMsg = "bad close error for test"
)

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
	t.Parallel()
	algos := []CompressionType{CompressionTypeGzip}
	t.Run("bad read body", func(t *testing.T) {
		t.Parallel()
		_, _, err := compressBody(algos, io.NopCloser(badReadBody()))
		require.Error(t, err)
		require.Equal(t, err.Error(), badReadMsg)
	})

	t.Run("bad close body", func(t *testing.T) {
		t.Parallel()
		_, _, err := compressBody(algos, badCloseBody())
		require.Error(t, err)
		require.Equal(t, err.Error(), badCloseMsg)
	})
}

func TestMakeRequestError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Run("bad compression algorithm body", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequest(http.MethodGet, "https://wont.be.used", nil)

		require.NoError(t, err)
		badCompressionType := CompressionType(13)
		require.False(t, badCompressionType.IsACompressionType())
		preq := &ParsedHTTPRequest{
			Req:          req,
			Body:         new(bytes.Buffer),
			Compressions: []CompressionType{badCompressionType},
		}
		state := &lib.State{
			Transport: http.DefaultTransport,
			Logger:    logrus.New(),
			Tags:      lib.NewVUStateTags(metrics.NewRegistry().RootTagSet()),
		}
		_, err = MakeRequest(ctx, state, preq)
		require.Error(t, err)
		require.Equal(t, err.Error(), "unknown compressionType CompressionType(13)")
	})

	t.Run("invalid upgrade response", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			Transport: srv.Client().Transport,
			Logger:    logger,
			Tags:      lib.NewVUStateTags(metrics.NewRegistry().RootTagSet()),
		}
		req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
		preq := &ParsedHTTPRequest{
			Req:         req,
			URL:         &URL{u: req.URL},
			Body:        new(bytes.Buffer),
			Timeout:     10 * time.Second,
			TagsAndMeta: state.Tags.GetCurrentValues(),
		}

		res, err := MakeRequest(ctx, state, preq)

		assert.Nil(t, res)
		assert.EqualError(t, err, "unsupported response status: 101 Switching Protocols")
	})
}

func TestResponseStatus(t *testing.T) {
	t.Parallel()
	t.Run("response status", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name                     string
			statusCode               int
			statusCodeExpected       int
			statusCodeStringExpected string
		}{
			{"status 200", 200, 200, "200 OK"},
			{"status 201", 201, 201, "201 Created"},
			{"status 202", 202, 202, "202 Accepted"},
			{"status 203", 203, 203, "203 Non-Authoritative Information"},
			{"status 204", 204, 204, "204 No Content"},
			{"status 205", 205, 205, "205 Reset Content"},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tc.statusCode)
				}))
				defer server.Close()
				logger := logrus.New()
				logger.Level = logrus.DebugLevel
				samples := make(chan<- metrics.SampleContainer, 1)
				registry := metrics.NewRegistry()
				state := &lib.State{
					Transport:      server.Client().Transport,
					Logger:         logger,
					Samples:        samples,
					BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
					Tags:           lib.NewVUStateTags(registry.RootTagSet()),
				}
				req, err := http.NewRequest(http.MethodGet, server.URL, nil)
				require.NoError(t, err)

				preq := &ParsedHTTPRequest{
					Req:          req,
					URL:          &URL{u: req.URL},
					Body:         new(bytes.Buffer),
					Timeout:      10 * time.Second,
					ResponseType: ResponseTypeNone,
					TagsAndMeta:  state.Tags.GetCurrentValues(),
				}

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				res, err := MakeRequest(ctx, state, preq)
				require.NoError(t, err)
				assert.Equal(t, tc.statusCodeExpected, res.Status)
				assert.Equal(t, tc.statusCodeStringExpected, res.StatusText)
			})
		}
	})
}

func TestURL(t *testing.T) {
	t.Parallel()
	t.Run("Clean", func(t *testing.T) {
		t.Parallel()
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
				t.Parallel()
				u, err := url.Parse(tc.url)
				require.NoError(t, err)
				ut := URL{u: u, URL: tc.url}
				require.Equal(t, tc.expected, ut.Clean())
			})
		}
	})
}

func TestMakeRequestTimeoutInTheMiddle(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Length", "100000")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	samples := make(chan metrics.SampleContainer, 10)
	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	registry := metrics.NewRegistry()
	state := &lib.State{
		Options: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Transport:      srv.Client().Transport,
		Samples:        samples,
		Logger:         logger,
		BufferPool:     lib.NewBufferPool(),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	preq := &ParsedHTTPRequest{
		Req:              req,
		URL:              &URL{u: req.URL, URL: srv.URL},
		Body:             new(bytes.Buffer),
		Timeout:          50 * time.Millisecond,
		ResponseCallback: func(i int) bool { return i == 0 },
		TagsAndMeta:      state.Tags.GetCurrentValues(),
	}

	res, err := MakeRequest(ctx, state, preq)
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, samples, 1)
	sampleCont := <-samples
	allSamples := sampleCont.GetSamples()
	require.Len(t, allSamples, 9)
	expTags := map[string]string{
		"error":             "request timeout",
		"error_code":        "1050",
		"status":            "0",
		"expected_response": "true", // we wait for status code 0
		"method":            "GET",
		"url":               srv.URL,
		"name":              srv.URL,
	}
	for _, s := range allSamples {
		assert.Equal(t, expTags, s.Tags.Map())
		assert.Nil(t, s.Metadata)
	}
}

func BenchmarkWrapDecompressionError(b *testing.B) {
	err := errors.New("error")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = wrapDecompressionError(err)
	}
}

func TestTrailFailed(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(httpbin.New().Handler())
	t.Cleanup(srv.Close)

	testCases := map[string]struct {
		responseCallback func(int) bool
		failed           null.Bool
	}{
		"null responsecallback": {responseCallback: nil, failed: null.NewBool(false, false)},
		"unexpected response":   {responseCallback: func(int) bool { return false }, failed: null.NewBool(true, true)},
		"expected response":     {responseCallback: func(int) bool { return true }, failed: null.NewBool(false, true)},
	}
	for name, testCase := range testCases {
		responseCallback := testCase.responseCallback
		failed := testCase.failed

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			samples := make(chan metrics.SampleContainer, 10)
			logger := logrus.New()
			logger.Level = logrus.DebugLevel
			registry := metrics.NewRegistry()
			state := &lib.State{
				Options: lib.Options{
					SystemTags: &metrics.DefaultSystemTagSet,
				},
				Transport:      srv.Client().Transport,
				Samples:        samples,
				Logger:         logger,
				BufferPool:     lib.NewBufferPool(),
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Tags:           lib.NewVUStateTags(registry.RootTagSet()),
			}
			req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
			preq := &ParsedHTTPRequest{
				Req:              req,
				URL:              &URL{u: req.URL, URL: srv.URL},
				Body:             new(bytes.Buffer),
				Timeout:          10 * time.Millisecond,
				ResponseCallback: responseCallback,
				TagsAndMeta:      state.Tags.GetCurrentValues(),
			}
			res, err := MakeRequest(ctx, state, preq)

			require.NoError(t, err)
			require.NotNil(t, res)
			require.Len(t, samples, 1)
			sample := <-samples

			trail, ok := sample.(*Trail)
			require.True(t, ok)

			require.Equal(t, failed, trail.Failed)

			var httpReqFailedSampleValue null.Bool
			for _, s := range sample.GetSamples() {
				if s.Metric.Name == metrics.HTTPReqFailedName {
					httpReqFailedSampleValue.Valid = true
					if s.Value == 1.0 {
						httpReqFailedSampleValue.Bool = true
					}
				}
			}
			require.Equal(t, failed, httpReqFailedSampleValue)
		})
	}
}

func TestMakeRequestDialTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("dial timeout doesn't get returned on windows") // or we don't match it correctly
	}
	t.Parallel()
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr()
	defer func() {
		require.NoError(t, ln.Close())
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	samples := make(chan metrics.SampleContainer, 10)
	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	registry := metrics.NewRegistry()
	state := &lib.State{
		Options: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 1 * time.Microsecond,
			}).DialContext,
		},
		Samples:        samples,
		Logger:         logger,
		BufferPool:     lib.NewBufferPool(),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}

	req, _ := http.NewRequest(http.MethodGet, "http://"+addr.String(), nil)
	preq := &ParsedHTTPRequest{
		Req:              req,
		URL:              &URL{u: req.URL, URL: req.URL.String()},
		Body:             new(bytes.Buffer),
		Timeout:          500 * time.Millisecond,
		ResponseCallback: func(i int) bool { return i == 0 },
		TagsAndMeta:      state.Tags.GetCurrentValues(),
	}

	res, err := MakeRequest(ctx, state, preq)
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, samples, 1)
	sampleCont := <-samples
	allSamples := sampleCont.GetSamples()
	require.Len(t, allSamples, 9)
	expTags := map[string]string{
		"error":             "dial: i/o timeout",
		"error_code":        "1211",
		"status":            "0",
		"expected_response": "true", // we wait for status code 0
		"method":            "GET",
		"url":               req.URL.String(),
		"name":              req.URL.String(),
	}
	for _, s := range allSamples {
		assert.Equal(t, expTags, s.Tags.Map())
		assert.Nil(t, s.Metadata)
	}
}

func TestMakeRequestTimeoutInTheBegining(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	samples := make(chan metrics.SampleContainer, 10)
	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	registry := metrics.NewRegistry()
	state := &lib.State{
		Options: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Transport:      srv.Client().Transport,
		Samples:        samples,
		Logger:         logger,
		BufferPool:     lib.NewBufferPool(),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	preq := &ParsedHTTPRequest{
		Req:              req,
		URL:              &URL{u: req.URL, URL: srv.URL},
		Body:             new(bytes.Buffer),
		Timeout:          50 * time.Millisecond,
		ResponseCallback: func(i int) bool { return i == 0 },
		TagsAndMeta:      state.Tags.GetCurrentValues(),
	}

	res, err := MakeRequest(ctx, state, preq)
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, samples, 1)
	sampleCont := <-samples
	allSamples := sampleCont.GetSamples()
	require.Len(t, allSamples, 9)
	expTags := map[string]string{
		"error":             "request timeout",
		"error_code":        "1050",
		"status":            "0",
		"expected_response": "true", // we wait for status code 0
		"method":            "GET",
		"url":               srv.URL,
		"name":              srv.URL,
	}
	for _, s := range allSamples {
		assert.Equal(t, expTags, s.Tags.Map())
		assert.Nil(t, s.Metadata)
	}
}

func TestMakeRequestRPSLimit(t *testing.T) {
	t.Parallel()
	var requests int64
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&requests, 1)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	samples := make(chan metrics.SampleContainer, 10)
	go func() {
		for {
			select {
			case <-samples:
			case <-ctx.Done():
				return
			}
		}
	}()

	logger := logrus.New()
	logger.Out = io.Discard

	registry := metrics.NewRegistry()
	state := &lib.State{
		Options: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		RPSLimit:       rate.NewLimiter(rate.Limit(1), 1),
		Transport:      ts.Client().Transport,
		Samples:        samples,
		Logger:         logger,
		BufferPool:     lib.NewBufferPool(),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}

	timer := time.NewTimer(3 * time.Second)
	for {
		select {
		case <-timer.C:
			timer.Stop()
			val := atomic.LoadInt64(&requests)
			assert.NotEmpty(t, val)
			assert.InDelta(t, val, 3, 3)
			return
		default:
			req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
			preq := &ParsedHTTPRequest{
				Req:         req,
				URL:         &URL{u: req.URL, URL: ts.URL, Name: ts.URL},
				Timeout:     10 * time.Millisecond,
				TagsAndMeta: state.Tags.GetCurrentValues(),
			}
			_, err := MakeRequest(ctx, state, preq)
			require.NoError(t, err)
		}
	}
}
