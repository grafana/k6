package cloudapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/lib/testutils"
)

// newProvisionTestServer creates a httptest.Server routing requests by path to the provided
// handlers, and returns a configured Client pointing at the server.
func newProvisionTestServer(t *testing.T, handlers map[string]http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()

	mux := http.NewServeMux()
	for path, handler := range handlers {
		mux.HandleFunc(path, handler)
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "1.0", 5*time.Second)
	require.NoError(t, err)
	client.SetStackID(99)

	return srv, client
}

// makeLoadTestResponse builds a minimal LoadTestApiModel JSON-serialisable value.
func makeLoadTestResponse(id int32, projectID int32, name string) map[string]any { //nolint:unparam
	now := time.Now().UTC().Format(time.RFC3339)
	return map[string]any{
		"id":                   id,
		"project_id":           projectID,
		"name":                 name,
		"baseline_test_run_id": nil,
		"created":              now,
		"updated":              now,
	}
}

// makeSLEResponse builds the start_local_execution response body.
func makeSLEResponse(testRunID int64, archiveUploadURL *string, detailsURL string) map[string]any { //nolint:unparam
	return map[string]any{
		"test_run_id":               testRunID,
		"archive_upload_url":        archiveUploadURL,
		"test_run_details_page_url": detailsURL,
		"runtime_config": map[string]any{
			"test_run_token": "scoped-tok",
			"metrics": map[string]any{
				"push_url": "https://ingest.k6.io/v2/metrics/42",
			},
			"traces": map[string]any{"push_url": "https://traces.k6.io"},
			"files":  map[string]any{"push_url": "https://files.k6.io"},
			"logs":   map[string]any{"push_url": "https://logs.k6.io", "tail_url": "https://logs-tail.k6.io"},
		},
	}
}

// TestProvisionLocalExecution_FullFlow verifies the 4-step provisioning flow end-to-end:
// CreateOrFindLoadTest → StartLocalExecution → UploadArchive → WaitForTestRunReady.
func TestProvisionLocalExecution_FullFlow(t *testing.T) {
	t.Parallel()

	var (
		createCalls  atomic.Int32
		startCalls   atomic.Int32
		uploadCalled atomic.Int32
		pollCalls    atomic.Int32
	)

	// uploadURLCh lets the start_local_execution handler receive the server URL
	// after the server is started (avoiding a chicken-and-egg problem).
	uploadURLCh := make(chan string, 1)

	handlers := map[string]http.HandlerFunc{
		"/cloud/v6/projects/99/load_tests": func(w http.ResponseWriter, r *http.Request) {
			createCalls.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			writeJSON(t, w, http.StatusCreated, makeLoadTestResponse(1234, 99, "test"))
		},
		"/provisioning/v1/load_tests/1234/start_local_execution": func(w http.ResponseWriter, r *http.Request) {
			startCalls.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			uploadURL := <-uploadURLCh
			writeJSON(t, w, http.StatusOK, makeSLEResponse(42, &uploadURL, "https://app.k6.io/runs/42"))
		},
		"/upload": func(w http.ResponseWriter, r *http.Request) {
			uploadCalled.Add(1)
			assert.Equal(t, http.MethodPut, r.Method)
			w.WriteHeader(http.StatusOK)
		},
		"/cloud/v6/test_runs/42": func(w http.ResponseWriter, _ *http.Request) {
			pollCalls.Add(1)
			writeJSON(t, w, http.StatusOK, makeTestRunResponse(StatusInitializing, nil))
		},
	}

	srv, client := newProvisionTestServer(t, handlers)

	// Provide the upload URL using the server's actual URL.
	uploadURLCh <- srv.URL + "/upload"

	arc := newTestArchive(t)
	params := ProvisionParams{
		Name:          "test",
		ProjectID:     99,
		MaxVUs:        5,
		TotalDuration: 60,
		Options:       map[string]any{},
		Archive:       arc,
	}

	result, err := client.ProvisionLocalExecution(t.Context(), params)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(42), result.TestRunID)
	assert.Equal(t, "https://ingest.k6.io/v2/metrics/42", result.RuntimeConfig.Metrics.PushURL)
	assert.Equal(t, "scoped-tok", result.RuntimeConfig.TestRunToken)
	assert.Equal(t, "https://app.k6.io/runs/42", result.TestRunDetailsPageURL)

	assert.Equal(t, int32(1), createCalls.Load(), "CreateOrFindLoadTest must be called exactly once")
	assert.Equal(t, int32(1), startCalls.Load(), "StartLocalExecution must be called exactly once")
	assert.Equal(t, int32(1), uploadCalled.Load(), "S3 upload must be called exactly once")
	assert.GreaterOrEqual(t, pollCalls.Load(), int32(1), "poll endpoint must be called at least once")
}

// TestProvisionLocalExecution_ArchiveNilSkipsUploadStillPolls verifies that when Archive is nil
// (--no-archive-upload), archive_size is sent as null so the backend skips waiting for an upload
// and goes straight to validation. The S3 upload endpoint is never called, but polling still
// happens because the test run may be queued before becoming ready.
func TestProvisionLocalExecution_ArchiveNilSkipsUploadStillPolls(t *testing.T) {
	t.Parallel()

	var (
		uploadCalled atomic.Int32
		pollCalls    atomic.Int32
		capturedBody []byte
	)

	handlers := map[string]http.HandlerFunc{
		"/cloud/v6/projects/99/load_tests": func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusCreated, makeLoadTestResponse(1234, 99, "test"))
		},
		"/provisioning/v1/load_tests/1234/start_local_execution": func(w http.ResponseWriter, r *http.Request) {
			capturedBody, _ = io.ReadAll(r.Body)
			writeJSON(t, w, http.StatusOK, makeSLEResponse(42, nil, "https://app.k6.io/runs/42"))
		},
		"/upload": func(w http.ResponseWriter, _ *http.Request) {
			uploadCalled.Add(1)
			w.WriteHeader(http.StatusOK)
		},
		"/cloud/v6/test_runs/42": func(w http.ResponseWriter, _ *http.Request) {
			pollCalls.Add(1)
			writeJSON(t, w, http.StatusOK, makeTestRunResponse(StatusInitializing, nil))
		},
	}

	_, client := newProvisionTestServer(t, handlers)

	params := ProvisionParams{
		Name:          "test",
		ProjectID:     99,
		MaxVUs:        5,
		TotalDuration: 60,
		Options:       map[string]any{},
		Archive:       nil, // --no-archive-upload
	}

	result, err := client.ProvisionLocalExecution(t.Context(), params)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(42), result.TestRunID)
	assert.Equal(t, int32(0), uploadCalled.Load(), "S3 upload must NOT be called when Archive is nil")
	assert.Equal(t, int32(1), pollCalls.Load(), "poll must be called even when Archive is nil (test run may be queued)")

	// archive_size must be explicitly null, not omitted, so the backend knows
	// no archive is expected and can skip waiting for an upload.
	var body map[string]any
	require.NoError(t, json.Unmarshal(capturedBody, &body))
	archiveSize, exists := body["archive_size"]
	assert.True(t, exists, "archive_size field must be present in request body")
	assert.Nil(t, archiveSize, "archive_size must be null when no archive is uploaded")
}

// TestProvisionLocalExecution_PollAbortedNoNotify verifies that when WaitForTestRunReady
// returns an aborted error, ProvisionLocalExecution surfaces the error without calling notify.
func TestProvisionLocalExecution_PollAbortedNoNotify(t *testing.T) {
	t.Parallel()

	uploadURLCh := make(chan string, 1)
	var notifyCalled atomic.Int32

	handlers := map[string]http.HandlerFunc{
		"/cloud/v6/projects/99/load_tests": func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusCreated, makeLoadTestResponse(1234, 99, "test"))
		},
		"/provisioning/v1/load_tests/1234/start_local_execution": func(w http.ResponseWriter, _ *http.Request) {
			uploadURL := <-uploadURLCh
			writeJSON(t, w, http.StatusOK, makeSLEResponse(42, &uploadURL, "https://app.k6.io/runs/42"))
		},
		"/upload": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
		"/cloud/v6/test_runs/42": func(w http.ResponseWriter, _ *http.Request) {
			history := []StatusEvent{
				{Status: StatusAborted, Message: "out of credit"},
			}
			writeJSON(t, w, http.StatusOK, makeTestRunResponse(StatusAborted, history))
		},
		// This handler must NOT be called by ProvisionLocalExecution.
		"/provisioning/v1/test_runs/42/notify": func(w http.ResponseWriter, _ *http.Request) {
			notifyCalled.Add(1)
			t.Errorf("notify endpoint must NOT be called by ProvisionLocalExecution")
			w.WriteHeader(http.StatusNoContent)
		},
	}

	srv, client := newProvisionTestServer(t, handlers)
	uploadURLCh <- srv.URL + "/upload"

	arc := newTestArchive(t)
	params := ProvisionParams{
		Name:          "test",
		ProjectID:     99,
		MaxVUs:        5,
		TotalDuration: 60,
		Options:       map[string]any{},
		Archive:       arc,
	}

	result, err := client.ProvisionLocalExecution(t.Context(), params)
	require.Error(t, err, "ProvisionLocalExecution must return an error when poll returns aborted")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "aborted", "error must mention aborted")
	assert.Equal(t, int32(0), notifyCalled.Load(), "notify must NOT be called by ProvisionLocalExecution")
}

// TestProvisionLocalExecution_ArchiveSerialisedOnce verifies that ProvisionLocalExecution
// serialises the archive only once and passes the resulting bytes directly to the S3
// upload, rather than re-serialising for the upload.
func TestProvisionLocalExecution_ArchiveSerialisedOnce(t *testing.T) {
	t.Parallel()

	var (
		uploadCalls  atomic.Int32
		capturedBody []byte
	)

	uploadURLCh := make(chan string, 1)

	handlers := map[string]http.HandlerFunc{
		"/cloud/v6/projects/99/load_tests": func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusCreated, makeLoadTestResponse(1234, 99, "test"))
		},
		"/provisioning/v1/load_tests/1234/start_local_execution": func(w http.ResponseWriter, _ *http.Request) {
			uploadURL := <-uploadURLCh
			writeJSON(t, w, http.StatusOK, makeSLEResponse(42, &uploadURL, "https://app.k6.io/runs/42"))
		},
		"/upload": func(w http.ResponseWriter, r *http.Request) {
			uploadCalls.Add(1)
			var err error
			capturedBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusOK)
		},
		"/cloud/v6/test_runs/42": func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusOK, makeTestRunResponse(StatusInitializing, nil))
		},
	}

	srv, client := newProvisionTestServer(t, handlers)
	uploadURLCh <- srv.URL + "/upload"

	arc := newTestArchive(t)
	result, err := client.ProvisionLocalExecution(t.Context(), ProvisionParams{
		Name:          "test",
		ProjectID:     99,
		MaxVUs:        5,
		TotalDuration: 60,
		Options:       map[string]any{},
		Archive:       arc,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(1), uploadCalls.Load(), "archive must be uploaded exactly once")
	assertArchiveEqual(t, arc, capturedBody)
}

// TestProvisionLocalExecution_NoNotifyBeforeTestRunID verifies that when the context is cancelled
// during CreateOrFindLoadTest (before a test_run_id is obtained), no notify call is attempted
// and the returned error wraps context.Canceled.
func TestProvisionLocalExecution_NoNotifyBeforeTestRunID(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var notifyCalled atomic.Int32

	// unblockCreate is used to signal the handler that the context has been cancelled,
	// allowing the server goroutine to return cleanly.
	unblockCreate := make(chan struct{})

	handlers := map[string]http.HandlerFunc{
		"/cloud/v6/projects/99/load_tests": func(w http.ResponseWriter, _ *http.Request) {
			// Cancel the client context to simulate Ctrl+C during CreateOrFindLoadTest.
			cancel()
			// Block until the test unblocks us so the server goroutine can return cleanly.
			<-unblockCreate
			w.WriteHeader(http.StatusServiceUnavailable)
		},
		"/provisioning/v1/test_runs/42/notify": func(w http.ResponseWriter, _ *http.Request) {
			notifyCalled.Add(1)
			t.Errorf("notify must NOT be called when there is no test_run_id yet")
			w.WriteHeader(http.StatusNoContent)
		},
	}

	_, client := newProvisionTestServer(t, handlers)
	// Unblock the server handler when the test finishes.
	t.Cleanup(func() { close(unblockCreate) })

	params := ProvisionParams{
		Name:      "test",
		ProjectID: 99,
		MaxVUs:    5,
		Options:   map[string]any{},
		Archive:   nil,
	}

	result, err := client.ProvisionLocalExecution(ctx, params)
	require.Error(t, err)
	assert.Nil(t, result)

	assert.ErrorIs(t, err, context.Canceled, "error must be (or wrap) context.Canceled")
	assert.Equal(t, int32(0), notifyCalled.Load(), "notify must NOT be called before test_run_id exists")
}
