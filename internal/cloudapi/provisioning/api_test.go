package provisioning

import (
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	provtest "go.k6.io/k6/v2/internal/cloudapi/provisioning/test"
	"go.k6.io/k6/v2/internal/lib/testutils"
)

func TestStartLocalExecution(t *testing.T) {
	t.Parallel()

	var (
		method     string
		path       string
		body       []byte
		idempKey   string
		authHeader string
		stackIDHdr string
		userAgent  string
	)

	srv := provtest.NewServer(t)
	srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		idempKey = r.Header.Get("K6-Idempotency-Key")
		authHeader = r.Header.Get("Authorization")
		stackIDHdr = r.Header.Get("X-Stack-Id")
		userAgent = r.Header.Get("User-Agent")
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}

		resp := provtest.DefaultStartLocalExecutionResponse()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	opts := json.RawMessage(`{"vus":10,"duration":"30s"}`)
	archiveSize := int64(1000)
	req := StartLocalExecutionRequest{
		Options:       opts,
		MaxVUs:        50,
		TotalDuration: 630,
		ArchiveSize:   &archiveSize,
	}

	got, err := client.StartLocalExecution(t.Context(), provtest.DefaultLoadTestID, req)
	require.NoError(t, err)

	// --- Response assertions ---
	assert.Equal(t, provtest.DefaultTestRunID, got.TestRunID)
	require.NotNil(t, got.ArchiveUploadURL)
	assert.Contains(t, *got.ArchiveUploadURL, "s3.amazonaws.com")
	assert.Contains(t, got.TestRunDetailsPageURL, "/runs/")
	assert.Equal(t, "https://ingest.k6.io/v1/metrics", got.RuntimeConfig.Metrics.PushURL)
	assert.Equal(t, "test-run-token-abc", got.RuntimeConfig.TestRunToken)
	assert.Equal(t, "https://api.k6.io/provisioning/v1/test_runs/123/decrypt_secret?name={key}", got.RuntimeConfig.Secrets.Endpoint)
	assert.Equal(t, "plaintext", got.RuntimeConfig.Secrets.ResponsePath)

	// Metrics tuning fields.
	require.NotNil(t, got.RuntimeConfig.Metrics.PushInterval)
	assert.Equal(t, "2s", *got.RuntimeConfig.Metrics.PushInterval)
	require.NotNil(t, got.RuntimeConfig.Metrics.PushConcurrency)
	assert.Equal(t, int32(5), *got.RuntimeConfig.Metrics.PushConcurrency)
	require.NotNil(t, got.RuntimeConfig.Metrics.AggregationPeriod)
	assert.Equal(t, "3s", *got.RuntimeConfig.Metrics.AggregationPeriod)
	require.NotNil(t, got.RuntimeConfig.Metrics.AggregationWaitPeriod)
	assert.Equal(t, "1s", *got.RuntimeConfig.Metrics.AggregationWaitPeriod)
	require.NotNil(t, got.RuntimeConfig.Metrics.MaxSamplesPerPackage)
	assert.Equal(t, int32(2000), *got.RuntimeConfig.Metrics.MaxSamplesPerPackage)

	// --- Request assertions ---
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "/provisioning/v1/load_tests/456/start_local_execution", path)
	assert.Equal(t, "Bearer test-token", authHeader)
	assert.Equal(t, "7", stackIDHdr)
	assert.Equal(t, "k6cloud/0.42.0", userAgent)
	assert.Regexp(t, `^[0-9a-f]+$`, idempKey)
	assert.GreaterOrEqual(t, len(idempKey), 1)

	// Request body shape.
	var reqBody map[string]any
	require.NoError(t, json.Unmarshal(body, &reqBody))
	assert.Equal(t, float64(50), reqBody["max_vus"])
	assert.Equal(t, float64(630), reqBody["total_duration"])
	assert.Equal(t, float64(1000), reqBody["archive_size"])

	reqOpts, ok := reqBody["options"].(map[string]any)
	require.True(t, ok, "options should be an object")
	assert.Equal(t, float64(10), reqOpts["vus"])
	assert.Equal(t, "30s", reqOpts["duration"])
}

func TestStartLocalExecution_NoArchive_ArchiveSizeNull(t *testing.T) {
	t.Parallel()

	var body []byte

	srv := provtest.NewServer(t)
	srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, r *http.Request) {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		resp := provtest.DefaultStartLocalExecutionResponse()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	req := StartLocalExecutionRequest{
		Options:       json.RawMessage(`{"vus":1}`),
		MaxVUs:        1,
		TotalDuration: 60,
		ArchiveSize:   nil, // no archive
	}

	_, err = client.StartLocalExecution(t.Context(), provtest.DefaultLoadTestID, req)
	require.NoError(t, err)

	// Verify archive_size is explicit JSON null (not omitted, not zero).
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &raw))

	archiveRaw, exists := raw["archive_size"]
	require.True(t, exists, "archive_size key must be present in request body")
	assert.Equal(t, "null", string(archiveRaw), "archive_size must be JSON null")
}

func TestStartLocalExecution_5xxRetried(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := provtest.NewServer(t)
	srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":"server_error","message":"temporary"}}`))
			return
		}
		resp := provtest.DefaultStartLocalExecutionResponse()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	req := StartLocalExecutionRequest{
		Options:       json.RawMessage(`{"vus":1}`),
		MaxVUs:        1,
		TotalDuration: 60,
		ArchiveSize:   nil,
	}

	got, err := client.StartLocalExecution(t.Context(), provtest.DefaultLoadTestID, req)
	require.NoError(t, err)
	assert.Equal(t, provtest.DefaultTestRunID, got.TestRunID)
	assert.Equal(t, int32(2), attempts.Load(), "expected exactly 2 attempts (1 failure + 1 success)")
}

func TestStartLocalExecution_4xxNotRetried(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := provtest.NewServer(t)
	srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"validation_error","message":"invalid options"}}`))
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	req := StartLocalExecutionRequest{
		Options:       json.RawMessage(`{"vus":1}`),
		MaxVUs:        1,
		TotalDuration: 60,
		ArchiveSize:   nil,
	}

	_, err = client.StartLocalExecution(t.Context(), provtest.DefaultLoadTestID, req)
	require.Error(t, err)

	var respErr *ResponseError
	require.ErrorAs(t, err, &respErr)
	assert.Equal(t, http.StatusBadRequest, respErr.StatusCode)
	assert.Equal(t, int32(1), attempts.Load(), "4xx must not be retried")
}
