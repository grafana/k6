/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
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
	algos := []CompressionType{CompressionTypeGzip}
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("bad compression algorithm body", func(t *testing.T) {
		req, err := http.NewRequest("GET", "https://wont.be.used", nil)

		require.NoError(t, err)
		badCompressionType := CompressionType(13)
		require.False(t, badCompressionType.IsACompressionType())
		preq := &ParsedHTTPRequest{
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
		preq := &ParsedHTTPRequest{
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

func TestResponseStatus(t *testing.T) {
	t.Run("response status", func(t *testing.T) {
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
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tc.statusCode)
				}))
				defer server.Close()
				logger := logrus.New()
				logger.Level = logrus.DebugLevel
				state := &lib.State{
					Options:   lib.Options{RunTags: &stats.SampleTags{}},
					Transport: server.Client().Transport,
					Logger:    logger,
					Samples:   make(chan<- stats.SampleContainer, 1),
				}
				ctx := lib.WithState(context.Background(), state)
				req, err := http.NewRequest("GET", server.URL, nil)
				require.NoError(t, err)

				preq := &ParsedHTTPRequest{
					Req:          req,
					URL:          &URL{u: req.URL},
					Body:         new(bytes.Buffer),
					Timeout:      10 * time.Second,
					ResponseType: ResponseTypeNone,
				}

				res, err := MakeRequest(ctx, preq)
				require.NoError(t, err)
				assert.Equal(t, tc.statusCodeExpected, res.Status)
				assert.Equal(t, tc.statusCodeStringExpected, res.StatusText)
			})
		}
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
		Options: lib.Options{
			RunTags:    &stats.SampleTags{},
			SystemTags: &stats.DefaultSystemTagSet,
		},
		Transport: srv.Client().Transport,
		Samples:   samples,
		Logger:    logger,
	}
	ctx = lib.WithState(ctx, state)
	req, _ := http.NewRequest("GET", srv.URL, nil)
	preq := &ParsedHTTPRequest{
		Req:     req,
		URL:     &URL{u: req.URL, URL: srv.URL},
		Body:    new(bytes.Buffer),
		Timeout: 10 * time.Millisecond,
	}

	res, err := MakeRequest(ctx, preq)
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, samples, 1)
	sampleCont := <-samples
	allSamples := sampleCont.GetSamples()
	require.Len(t, allSamples, 8)
	expTags := map[string]string{
		"error":      "context deadline exceeded",
		"error_code": "1000",
		"status":     "0",
		"method":     "GET",
		"url":        srv.URL,
		"name":       srv.URL,
	}
	for _, s := range allSamples {
		assert.Equal(t, expTags, s.Tags.CloneTags())
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
