package provisioning

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/errext"
	provtest "go.k6.io/k6/v2/internal/cloudapi/provisioning/test"
	v6 "go.k6.io/k6/v2/internal/cloudapi/v6"
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
		ArchiveSize:   archiveSize,
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

	// Logs config.
	assert.Equal(t, "https://logs.k6.io", got.RuntimeConfig.Logs.PushURL)
	assert.Equal(t, "info", got.RuntimeConfig.Logs.Level)
	assert.Equal(t, int32(900), got.RuntimeConfig.Logs.Limit)
	assert.Equal(t, "3s", got.RuntimeConfig.Logs.PushPeriodSeconds)
	assert.Equal(t, int32(10000), got.RuntimeConfig.Logs.MessageMaxSize)
	assert.Equal(t, []string{"lz", "level"}, got.RuntimeConfig.Logs.AllowedLabels)

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
		ArchiveSize:   0, // no archive
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
		ArchiveSize:   0,
	}

	got, err := client.StartLocalExecution(t.Context(), provtest.DefaultLoadTestID, req)
	require.NoError(t, err)
	assert.Equal(t, provtest.DefaultTestRunID, got.TestRunID)
	assert.Equal(t, int32(2), attempts.Load(), "expected exactly 2 attempts (1 failure + 1 success)")
}

func TestUploadArchive(t *testing.T) {
	t.Parallel()

	var (
		method        string
		reqPath       string
		contentType   string
		authHeader    string
		contentLength int64
		gotBody       []byte
		hitCount      atomic.Int32
	)

	srv := provtest.NewServer(t)
	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		method = r.Method
		reqPath = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		authHeader = r.Header.Get("Authorization")
		contentLength = r.ContentLength
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	archiveBody := []byte("fake-archive-tar-content-12345")
	err = client.UploadArchive(t.Context(), srv.PresignedUploadURL(), archiveBody)
	require.NoError(t, err)

	assert.Equal(t, http.MethodPut, method)
	assert.Equal(t, provtest.PresignedUploadPath, reqPath)
	assert.Equal(t, "application/x-tar", contentType)
	assert.Empty(t, authHeader, "presigned URLs must not carry an Authorization header")
	assert.Equal(t, archiveBody, gotBody)
	assert.Equal(t, int64(len(archiveBody)), contentLength)
	assert.Equal(t, int32(1), hitCount.Load(), "server should be hit exactly once")
}

func TestUploadArchive_RetriesOn5xx(t *testing.T) {
	t.Parallel()

	var (
		attempts atomic.Int32
		body1    []byte
		body2    []byte
	)

	srv := provtest.NewServer(t)
	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if attempts.Add(1) == 1 {
			body1 = b
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body2 = b
		w.WriteHeader(http.StatusOK)
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	archiveBody := []byte("archive-bytes-for-retry-test")
	err = client.UploadArchive(t.Context(), srv.PresignedUploadURL(), archiveBody)
	require.NoError(t, err)

	assert.Equal(t, int32(2), attempts.Load(), "expected exactly 2 attempts (1 failure + 1 success)")
	assert.Equal(t, body1, body2, "body replay should produce identical bytes on retry")
}

func TestUploadArchive_4xxNotRetried(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := provtest.NewServer(t)
	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Request has expired"))
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	archiveBody := []byte("archive-bytes-for-4xx-test")
	err = client.UploadArchive(t.Context(), srv.PresignedUploadURL(), archiveBody)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "403")
	assert.Contains(t, err.Error(), "Request has expired")
	assert.Equal(t, int32(1), attempts.Load(), "4xx must not be retried")
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
		ArchiveSize:   0,
	}

	_, err = client.StartLocalExecution(t.Context(), provtest.DefaultLoadTestID, req)
	require.Error(t, err)

	var respErr *ResponseError
	require.ErrorAs(t, err, &respErr)
	assert.Equal(t, http.StatusBadRequest, respErr.StatusCode)
	assert.Equal(t, int32(1), attempts.Load(), "4xx must not be retried")
}

func TestWaitForTestRunReady(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)
	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.WaitForTestRunReady(t.Context(), provtest.DefaultTestRunID, 1*time.Microsecond)
	require.NoError(t, err)

	assert.Equal(t, int32(1), srv.FetchTestRunHitCount(), "server should be hit exactly once")
}

func TestWaitForTestRunReady_PollsUntilInitializing(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)
	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{Status: v6.StatusCreated},
		{Status: v6.StatusQueued},
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.WaitForTestRunReady(t.Context(), provtest.DefaultTestRunID, 1*time.Microsecond)
	require.NoError(t, err)

	assert.Equal(t, int32(3), srv.FetchTestRunHitCount(), "server should be hit 3 times")
}

func TestWaitForTestRunReady_AbortedReturnsErrorWithMessage(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)
	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{
			Status: v6.StatusAborted,
			StatusHistory: []v6.StatusEvent{
				{Status: v6.StatusAborted, Message: "quota exceeded"},
			},
		},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.WaitForTestRunReady(t.Context(), provtest.DefaultTestRunID, 1*time.Microsecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quota exceeded")
}

func TestWaitForTestRunReady_AbortedNoHistoryReturnsGenericError(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)
	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{Status: v6.StatusAborted},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.WaitForTestRunReady(t.Context(), provtest.DefaultTestRunID, 1*time.Microsecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test run aborted before starting")
}

func TestWaitForTestRunReady_CompletedReturnsError(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)
	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{Status: v6.StatusCompleted},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.WaitForTestRunReady(t.Context(), provtest.DefaultTestRunID, 1*time.Microsecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "completed")
}

func TestWaitForTestRunReady_UnknownStatusKeepsPolling(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)
	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{Status: v6.Status("weird-state")},
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.WaitForTestRunReady(t.Context(), provtest.DefaultTestRunID, 1*time.Microsecond)
	require.NoError(t, err)

	assert.Equal(t, int32(2), srv.FetchTestRunHitCount(), "server should be hit twice")
}

func TestWaitForTestRunReady_LogsTransitionsOnce(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)
	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{Status: v6.StatusCreated},
		{Status: v6.StatusCreated},
		{Status: v6.StatusQueued},
		{Status: v6.StatusQueued},
		{Status: v6.StatusInitializing},
	})

	logger, hook := testutils.NewLoggerWithHook(t, logrus.InfoLevel)
	client, err := NewClient(logger, "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.WaitForTestRunReady(t.Context(), provtest.DefaultTestRunID, 1*time.Microsecond)
	require.NoError(t, err)

	entries := hook.Drain()
	// Filter to only info-level entries with a "status" field.
	var statusEntries []logrus.Entry
	for _, e := range entries {
		if e.Level == logrus.InfoLevel {
			if _, ok := e.Data["status"]; ok {
				statusEntries = append(statusEntries, e)
			}
		}
	}

	// Should have exactly 2 log entries: one for "created", one for "queued".
	// "initializing" is a terminal success — no log for it.
	require.Len(t, statusEntries, 2,
		"expected exactly 2 status transition log entries (created, queued)")
	assert.Equal(t, "Created", statusEntries[0].Data["status"])
	assert.Equal(t, "Queued", statusEntries[1].Data["status"])
}

func TestWaitForTestRunReady_ContextCancellation(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)
	// Server always returns "created" — polling never terminates on its own.
	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{Status: v6.StatusCreated},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	// Cancel after a small delay to let a few polls happen.
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	err = client.WaitForTestRunReady(ctx, provtest.DefaultTestRunID, 1*time.Microsecond)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestNotifyTestRunCompleted(t *testing.T) {
	t.Parallel()

	var (
		method      string
		reqPath     string
		authHeader  string
		contentType string
		userAgent   string
		body        []byte
		hitCount    atomic.Int32
	)

	srv := provtest.NewServer(t)
	srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		method = r.Method
		reqPath = r.URL.Path
		authHeader = r.Header.Get("Authorization")
		contentType = r.Header.Get("Content-Type")
		userAgent = r.Header.Get("User-Agent")
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.NotifyTestRunCompleted(t.Context(), provtest.DefaultTestRunID, "scoped-token-xyz", nil)
	require.NoError(t, err)

	// --- Request assertions ---
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "/provisioning/v1/test_runs/123/notify", reqPath)
	assert.Equal(t, "Bearer scoped-token-xyz", authHeader)
	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, "k6cloud/0.42.0", userAgent)
	assert.Equal(t, int32(1), hitCount.Load(), "server should be hit exactly once")

	// Request body shape: event_type present, error null.
	var reqBody map[string]any
	require.NoError(t, json.Unmarshal(body, &reqBody))
	assert.Equal(t, "script_execution_completed", reqBody["event_type"])
	assert.Nil(t, reqBody["error"])
}

func TestNotifyTestRunCompleted_WithErrorInRequestBody(t *testing.T) {
	t.Parallel()

	var body []byte

	srv := provtest.NewServer(t)
	srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, r *http.Request) {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	testErr := errext.WithAbortReasonIfNone(errors.New("TypeError: foo is not a function"), errext.AbortedByScriptError)
	err = client.NotifyTestRunCompleted(t.Context(), provtest.DefaultTestRunID, "scoped-token-xyz", testErr)
	require.NoError(t, err)

	// Request body should include the mapped error.
	var reqBody struct {
		EventType string `json:"event_type"`
		Error     *struct {
			Code   int    `json:"code"`
			Reason string `json:"reason"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &reqBody))
	assert.Equal(t, "script_execution_completed", reqBody.EventType)
	require.NotNil(t, reqBody.Error)
	assert.Equal(t, 8035, reqBody.Error.Code)
	assert.Contains(t, reqBody.Error.Reason, "TypeError")
}

func TestNotifyTestRunCompleted_4xxResponseReturnsError(t *testing.T) {
	t.Parallel()

	var hitCount atomic.Int32

	srv := provtest.NewServer(t)
	srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, _ *http.Request) {
		hitCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"validation_error","message":"bad event_type"}}`))
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.NotifyTestRunCompleted(t.Context(), provtest.DefaultTestRunID, "scoped-token-xyz", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "bad event_type")
	assert.Equal(t, int32(1), hitCount.Load(), "4xx must not be retried")
}

func TestNotifyTestRunCompleted_5xxRetried(t *testing.T) {
	t.Parallel()

	var hitCount atomic.Int32

	srv := provtest.NewServer(t)
	srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, _ *http.Request) {
		if hitCount.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	err = client.NotifyTestRunCompleted(t.Context(), provtest.DefaultTestRunID, "scoped-token-xyz", nil)
	require.NoError(t, err)
	assert.Equal(t, int32(2), hitCount.Load(), "expected exactly 2 attempts (1 failure + 1 success)")
}
