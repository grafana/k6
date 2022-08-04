package cloudapi

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/types"
)

func fprintf(t *testing.T, w io.Writer, format string, a ...interface{}) int {
	n, err := fmt.Fprintf(w, format, a...)
	require.NoError(t, err)
	return n
}

func TestCreateTestRun(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fprintf(t, w, `{"reference_id": "1", "config": {"aggregationPeriod": "2s"}}`)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)

	tr := &TestRun{
		Name: "test",
	}
	resp, err := client.CreateTestRun(tr)

	assert.Nil(t, err)
	assert.Equal(t, resp.ReferenceID, "1")
	assert.NotNil(t, resp.ConfigOverride)
	assert.True(t, resp.ConfigOverride.AggregationPeriod.Valid)
	assert.Equal(t, types.Duration(2*time.Second), resp.ConfigOverride.AggregationPeriod.Duration)
	assert.False(t, resp.ConfigOverride.AggregationMinSamples.Valid)
}

func TestFinished(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fprintf(t, w, "")
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

func TestAuthorizedError(t *testing.T) {
	t.Parallel()
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusForbidden)
		fprintf(t, w, `{"error": {"code": 5, "message": "Not allowed"}}`)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)

	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 1, called)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "(403/E5) Not allowed")
}

func TestDetailsError(t *testing.T) {
	t.Parallel()
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusForbidden)
		fprintf(t, w, `{"error": {"code": 0, "message": "Validation failed", "details": { "name": ["Shorter than minimum length 2."]}}}`)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)

	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 1, called)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "(403) Validation failed\n name: Shorter than minimum length 2.")
}

func TestClientRetry(t *testing.T) {
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
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)
	client.retryInterval = 1 * time.Millisecond
	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 3, called)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestClientRetrySuccessOnSecond(t *testing.T) {
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
			fprintf(t, w, `{"reference_id": "1"}`)
			return
		}
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := NewClient(testutils.NewLogger(t), "token", server.URL, "1.0", 1*time.Second)
	client.retryInterval = 1 * time.Millisecond
	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 2, called)
	assert.NotNil(t, resp)
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

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("Timeout", func(t *testing.T) {
		t.Parallel()

		timeout := 5 * time.Second
		c := NewClient(testutils.NewLogger(t), "token", "server-url", "1.0", 5*time.Second)
		assert.Equal(t, timeout, c.client.Timeout)
	})
}
