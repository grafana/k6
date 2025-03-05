package cloudapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"go.k6.io/k6/lib/fsext"

	"go.k6.io/k6/lib"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/types"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("Timeout", func(t *testing.T) {
		t.Parallel()

		timeout := 5 * time.Second
		c := NewClient(testutils.NewLogger(t), "token", "server-url", "1.0", 5*time.Second)
		assert.Equal(t, timeout, c.client.Timeout)
	})
}

func TestCreateTestRun(t *testing.T) {
	t.Parallel()

	t.Run("creating a test run should succeed", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			exp := `{"name":"test","vus":0,"thresholds":null,"duration":0}`
			assert.JSONEq(t, exp, string(b))

			fprint(t, w, `{"reference_id": "1", "config": {"aggregationPeriod": "2s"}}`)
		}))
		defer server.Close()

		client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)

		resp, err := client.CreateTestRun(&TestRun{
			Name: "test",
		})
		assert.NoError(t, err)

		assert.Equal(t, resp.ReferenceID, "1")
		assert.NotNil(t, resp.ConfigOverride)
		assert.True(t, resp.ConfigOverride.AggregationPeriod.Valid)
		assert.Equal(t, types.Duration(2*time.Second), resp.ConfigOverride.AggregationPeriod.Duration)
		assert.False(t, resp.ConfigOverride.MaxTimeSeriesInBatch.Valid)
	})

	t.Run("creating a test run with an archive should succeed", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse the multipart form data
			reader, err := r.MultipartReader()
			require.NoError(t, err)

			// Initialize a map to store the parsed form data
			formData := make(map[string]string)

			// Iterate through the parts
			for {
				part, nextErr := reader.NextPart()
				if errors.Is(nextErr, io.EOF) {
					break
				}
				require.NoError(t, nextErr)

				// Read the part content
				buf := new(bytes.Buffer)
				_, err = io.Copy(buf, part)
				require.NoError(t, err)

				// Store the part content in the map
				formData[part.FormName()] = buf.String()
			}

			assert.Equal(t, "test", formData["name"])
			assert.Equal(t, "0", formData["vus"])
			assert.Equal(t, "", formData["thresholds"])
			assert.Equal(t, "0", formData["duration"])

			fprint(t, w, `{"reference_id": "1", "config": {"aggregationPeriod": "2s"}}`)
		}))
		defer server.Close()

		client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)

		// Produce a test archive
		fs := fsext.NewMemMapFs()
		err := fsext.WriteFile(fs, "/path/to/a.js", []byte(`// a contents`), 0o644)
		require.NoError(t, err)

		arc := &lib.Archive{
			Type:        "js",
			K6Version:   build.Version,
			Options:     lib.Options{},
			FilenameURL: &url.URL{Scheme: "file", Path: "/path/to/a.js"},
			Data:        []byte(`// a contents`),
			PwdURL:      &url.URL{Scheme: "file", Path: "/path/to"},
			Filesystems: map[string]fsext.Fs{
				"file": fs,
			},
		}

		resp, err := client.CreateTestRun(&TestRun{
			Name:    "test",
			Archive: arc,
		})
		assert.NoError(t, err)

		assert.Equal(t, resp.ReferenceID, "1")
		assert.NotNil(t, resp.ConfigOverride)
		assert.True(t, resp.ConfigOverride.AggregationPeriod.Valid)
		assert.Equal(t, types.Duration(2*time.Second), resp.ConfigOverride.AggregationPeriod.Duration)
		assert.False(t, resp.ConfigOverride.MaxTimeSeriesInBatch.Valid)
	})

	t.Run("invalid authorization should fail", func(t *testing.T) {
		t.Parallel()
		called := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called++
			w.WriteHeader(http.StatusForbidden)
			fprint(t, w, `{"error": {"code": 5, "message": "Not allowed"}}`)
		}))
		defer server.Close()

		client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)

		resp, err := client.CreateTestRun(&TestRun{Name: "test"})

		assert.Equal(t, 1, called)
		assert.Nil(t, resp)
		assert.EqualError(t, err, "(403/E5) Not allowed")
	})

	t.Run("invalid payload should fail", func(t *testing.T) {
		t.Parallel()
		called := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called++
			w.WriteHeader(http.StatusForbidden)
			fprint(t, w, `{"error": {"code": 0, "message": "Validation failed", "details": { "name": ["Shorter than minimum length 2."]}}}`)
		}))
		defer server.Close()

		client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)

		resp, err := client.CreateTestRun(&TestRun{Name: "test"})

		assert.Equal(t, 1, called)
		assert.Nil(t, resp)
		assert.EqualError(t, err, "(403) Validation failed\n name: Shorter than minimum length 2.")
	})

	// FIXME (@oleiade): improve test name
	t.Run("retry should fail", func(t *testing.T) {
		t.Parallel()

		called := 0
		idempotencyKey := ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotK6IdempotencyKey := r.Header.Get(k6IdempotencyKeyHeader)
			if idempotencyKey == "" {
				idempotencyKey = gotK6IdempotencyKey
			}
			assert.NotEmpty(t, gotK6IdempotencyKey)
			assert.Equal(t, idempotencyKey, gotK6IdempotencyKey)
			called++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)
		client.retryInterval = 1 * time.Millisecond
		resp, err := client.CreateTestRun(&TestRun{Name: "test"})

		assert.Equal(t, 3, called)
		assert.Nil(t, resp)
		assert.NotNil(t, err)
	})

	// FIXME (@oleiade): improve test name
	t.Run("second retry should succeed", func(t *testing.T) {
		t.Parallel()

		called := 1
		idempotencyKey := ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotK6IdempotencyKey := r.Header.Get(k6IdempotencyKeyHeader)
			if idempotencyKey == "" {
				idempotencyKey = gotK6IdempotencyKey
			}
			assert.NotEmpty(t, gotK6IdempotencyKey)
			assert.Equal(t, idempotencyKey, gotK6IdempotencyKey)
			called++
			if called == 2 {
				fprint(t, w, `{"reference_id": "1"}`)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)
		client.retryInterval = 1 * time.Millisecond
		resp, err := client.CreateTestRun(&TestRun{Name: "test"})

		assert.Equal(t, 2, called)
		assert.NotNil(t, resp)
		assert.Nil(t, err)
	})
}

func TestFinished(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fprint(t, w, "")
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)

	thresholds := map[string]map[string]bool{
		"threshold": {
			"max < 10": true,
		},
	}
	err := client.TestFinished("1", thresholds, true, 0)

	assert.Nil(t, err)
}

func TestIdempotencyKey(t *testing.T) {
	t.Parallel()
	const idempotencyKey = "xxx"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotK6IdempotencyKey := r.Header.Get(k6IdempotencyKeyHeader)
		switch r.Method {
		case http.MethodPost:
			assert.NotEmpty(t, gotK6IdempotencyKey)
			assert.Equal(t, idempotencyKey, gotK6IdempotencyKey)
		default:
			assert.Empty(t, gotK6IdempotencyKey)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)
	client.retryInterval = 1 * time.Millisecond
	req, err := client.NewRequest(http.MethodPost, server.URL, nil)
	assert.NoError(t, err)
	req.Header.Set(k6IdempotencyKeyHeader, idempotencyKey)
	assert.NoError(t, client.Do(req, nil))

	req, err = client.NewRequest(http.MethodGet, server.URL, nil)
	assert.NoError(t, err)
	assert.NoError(t, client.Do(req, nil))
}

func fprint(t *testing.T, w io.Writer, format string) int {
	n, err := fmt.Fprint(w, format)
	require.NoError(t, err)
	return n
}
