package provisioning

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	provtest "go.k6.io/k6/v2/internal/cloudapi/provisioning/test"
	v6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/fsext"
)

const (
	testLoadTestID int64 = 42
	testTestRunID  int64 = 100
	testProjectID  int64 = 1
)

// callRecorder tracks the sequence of HTTP endpoint calls for test assertions.
type callRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (cr *callRecorder) record(name string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.calls = append(cr.calls, name)
}

func (cr *callRecorder) order() []string {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	return append([]string{}, cr.calls...)
}

func (cr *callRecorder) count(name string) int {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	n := 0
	for _, c := range cr.calls {
		if c == name {
			n++
		}
	}
	return n
}

func newTestArchive(t *testing.T) *lib.Archive {
	t.Helper()
	fs := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fs, "/a.js", []byte(`// test`), 0o644))
	return &lib.Archive{
		Type:        "js",
		K6Version:   "0.42.0",
		FilenameURL: &url.URL{Scheme: "file", Path: "/a.js"},
		PwdURL:      &url.URL{Scheme: "file", Path: "/"},
		Data:        []byte(`// test`),
		Filesystems: map[string]fsext.Fs{"file": fs},
	}
}

// writeCreateResponse writes a v6 create-load-test response using testLoadTestID.
func writeCreateResponse(w http.ResponseWriter) {
	res := k6cloud.NewLoadTestApiModelWithDefaults()
	res.SetId(testLoadTestID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		panic(fmt.Errorf("encoding create response: %w", err))
	}
}

// writeSLEResponse writes a StartLocalExecution response using
// testTestRunID and the given archive upload URL.
func writeSLEResponse(w http.ResponseWriter, uploadURL string) {
	resp := provtest.DefaultStartLocalExecutionResponse()
	resp.SetTestRunId(testTestRunID)
	u := uploadURL
	resp.ArchiveUploadUrl = *k6cloud.NewNullableString(&u)
	resp.SetTestRunDetailsPageUrl(fmt.Sprintf("https://app.k6.io/runs/%d", testTestRunID))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(fmt.Errorf("encoding SLE response: %w", err))
	}
}

func TestProvisionLocalExecution_WithArchive(t *testing.T) {
	t.Parallel()

	var rec callRecorder

	srv := provtest.NewServer(t)

	// 1. Create-or-find → load test ID 42.
	srv.HandleCreateLoadTest(testProjectID, func(w http.ResponseWriter, _ *http.Request) {
		rec.record("create")
		writeCreateResponse(w)
	})

	// 2. Start local execution → test run ID 100.
	srv.HandleStartLocalExecution(testLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		rec.record("start")
		writeSLEResponse(w, srv.PresignedUploadURL())
	})

	// 3. S3 presigned upload → 200.
	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		rec.record("upload")
		w.WriteHeader(http.StatusOK)
	})

	// 4. Test-run poll → "initializing" immediately.
	srv.HandleFetchTestRun(testTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	result, err := client.ProvisionLocalExecution(t.Context(), ProvisionParams{
		Name:          "my-test",
		ProjectID:     testProjectID,
		MaxVUs:        50,
		TotalDuration: 630,
		Options:       json.RawMessage(`{"vus":10,"duration":"30s"}`),
		Archive:       newTestArchive(t),
		PollInterval:  time.Microsecond,
	})
	require.NoError(t, err)

	// --- ProvisionResult assertions ---
	assert.Equal(t, testTestRunID, result.TestRunID)
	assert.Contains(t, result.TestRunDetailsPageURL, "/runs/100")
	assert.Equal(t, "https://ingest.k6.io/v1/metrics", result.RuntimeConfig.Metrics.PushURL)
	assert.Equal(t, "test-run-token-abc", result.RuntimeConfig.TestRunToken)
	assert.Equal(t,
		"https://api.k6.io/provisioning/v1/test_runs/123/decrypt_secret?name={key}",
		result.RuntimeConfig.Secrets.Endpoint)
	assert.Equal(t, "plaintext", result.RuntimeConfig.Secrets.ResponsePath)

	// --- Call counts ---
	assert.Equal(t, 1, rec.count("create"))
	assert.Equal(t, 1, rec.count("start"))
	assert.Equal(t, 1, rec.count("upload"))
	assert.GreaterOrEqual(t, srv.FetchTestRunHitCount(), int32(1))

	// --- Call order (create → start → upload; poll follows implicitly) ---
	calls := rec.order()
	require.Len(t, calls, 3)
	assert.Equal(t, []string{"create", "start", "upload"}, calls)
}

func TestProvisionLocalExecution_NoArchive(t *testing.T) {
	t.Parallel()

	var sleBody []byte

	srv := provtest.NewServer(t)

	srv.HandleCreateLoadTest(testProjectID, func(w http.ResponseWriter, _ *http.Request) {
		writeCreateResponse(w)
	})

	srv.HandleStartLocalExecution(testLoadTestID, func(w http.ResponseWriter, r *http.Request) {
		var err error
		sleBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusInternalServerError)
			return
		}
		writeSLEResponse(w, srv.PresignedUploadURL())
	})

	var uploadHits atomic.Int32
	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		uploadHits.Add(1)
		w.WriteHeader(http.StatusOK)
	})

	srv.HandleFetchTestRun(testTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	result, err := client.ProvisionLocalExecution(t.Context(), ProvisionParams{
		Name:          "my-test",
		ProjectID:     testProjectID,
		MaxVUs:        10,
		TotalDuration: 60,
		Options:       json.RawMessage(`{"vus":1}`),
		Archive:       nil,
		PollInterval:  time.Microsecond,
	})
	require.NoError(t, err)

	// archive_size must be explicit JSON null in the SLE request body.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(sleBody, &raw))
	archiveRaw, exists := raw["archive_size"]
	require.True(t, exists, "archive_size key must be present")
	assert.Equal(t, "null", string(archiveRaw))

	// S3 stub must NOT be called.
	assert.Equal(t, int32(0), uploadHits.Load(), "upload must not be called when archive is nil")

	// Polling must still happen (unconditional).
	assert.GreaterOrEqual(t, srv.FetchTestRunHitCount(), int32(1))

	// ProvisionResult populated.
	assert.Equal(t, testTestRunID, result.TestRunID)
	assert.NotEmpty(t, result.TestRunDetailsPageURL)
	assert.NotEmpty(t, result.RuntimeConfig.TestRunToken)
}

func TestProvisionLocalExecution_409ConflictPropagatesToFindByName(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)

	// Create: POST → 409; GET (findByName) → matching test with ID 42.
	srv.HandleCreateLoadTest(testProjectID, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"code": "conflict", "message": "already exists"},
			})
		case http.MethodGet:
			lt := k6cloud.NewLoadTestApiModelWithDefaults()
			lt.SetId(testLoadTestID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"value": []any{lt}})
		}
	})

	srv.HandleStartLocalExecution(testLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		writeSLEResponse(w, srv.PresignedUploadURL())
	})

	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv.HandleFetchTestRun(testTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	result, err := client.ProvisionLocalExecution(t.Context(), ProvisionParams{
		Name:          "my-test",
		ProjectID:     testProjectID,
		MaxVUs:        50,
		TotalDuration: 630,
		Options:       json.RawMessage(`{"vus":10}`),
		Archive:       newTestArchive(t),
		PollInterval:  time.Microsecond,
	})
	require.NoError(t, err)

	assert.Equal(t, testTestRunID, result.TestRunID)
	assert.Contains(t, result.TestRunDetailsPageURL, "/runs/100")
	assert.Equal(t, "test-run-token-abc", result.RuntimeConfig.TestRunToken)
}

func TestProvisionLocalExecution_ArchivePresentButNoUploadURL(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)

	srv.HandleCreateLoadTest(testProjectID, func(w http.ResponseWriter, _ *http.Request) {
		writeCreateResponse(w)
	})

	// start_local_execution returns no archive upload URL (null).
	srv.HandleStartLocalExecution(testLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		resp := provtest.DefaultStartLocalExecutionResponse()
		resp.SetTestRunId(testTestRunID)
		resp.ArchiveUploadUrl = *k6cloud.NewNullableString(nil)
		resp.SetTestRunDetailsPageUrl(fmt.Sprintf("https://app.k6.io/runs/%d", testTestRunID))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})

	// The upload endpoint must not be hit when no URL is returned.
	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("archive upload must not be attempted when no upload URL is returned")
		w.WriteHeader(http.StatusOK)
	})

	srv.HandleFetchTestRun(testTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	// Archive provided but API returns no upload URL: provisioning should
	// still succeed (a warning is logged) and skip the upload.
	result, err := client.ProvisionLocalExecution(t.Context(), ProvisionParams{
		Name:          "my-test",
		ProjectID:     testProjectID,
		MaxVUs:        50,
		TotalDuration: 630,
		Options:       json.RawMessage(`{"vus":10}`),
		Archive:       newTestArchive(t),
		PollInterval:  time.Microsecond,
	})
	require.NoError(t, err)
	assert.Equal(t, testTestRunID, result.TestRunID)
}

func TestProvisionLocalExecution_ErrorAtCreate(t *testing.T) {
	t.Parallel()

	var rec callRecorder

	srv := provtest.NewServer(t)

	srv.HandleCreateLoadTest(testProjectID, func(w http.ResponseWriter, _ *http.Request) {
		rec.record("create")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "server_error", "message": "internal error"},
		})
	})

	srv.HandleStartLocalExecution(testLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		rec.record("start")
		w.WriteHeader(http.StatusOK)
	})

	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		rec.record("upload")
		w.WriteHeader(http.StatusOK)
	})

	srv.HandleFetchTestRun(testTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	_, err = client.ProvisionLocalExecution(t.Context(), ProvisionParams{
		Name:          "my-test",
		ProjectID:     testProjectID,
		MaxVUs:        10,
		TotalDuration: 60,
		Options:       json.RawMessage(`{"vus":1}`),
		PollInterval:  time.Microsecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create or find load test")

	// Subsequent endpoints must not be called.
	assert.Equal(t, 0, rec.count("start"))
	assert.Equal(t, 0, rec.count("upload"))
	assert.Equal(t, int32(0), srv.FetchTestRunHitCount())
}

func TestProvisionLocalExecution_ErrorAtStart(t *testing.T) {
	t.Parallel()

	var rec callRecorder

	srv := provtest.NewServer(t)

	srv.HandleCreateLoadTest(testProjectID, func(w http.ResponseWriter, _ *http.Request) {
		rec.record("create")
		writeCreateResponse(w)
	})

	srv.HandleStartLocalExecution(testLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		rec.record("start")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"server_error","message":"start failed"}}`))
	})

	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		rec.record("upload")
		w.WriteHeader(http.StatusOK)
	})

	srv.HandleFetchTestRun(testTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	_, err = client.ProvisionLocalExecution(t.Context(), ProvisionParams{
		Name:          "my-test",
		ProjectID:     testProjectID,
		MaxVUs:        10,
		TotalDuration: 60,
		Options:       json.RawMessage(`{"vus":1}`),
		PollInterval:  time.Microsecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start local execution")

	// S3 + poll must not be called.
	assert.Equal(t, 0, rec.count("upload"))
	assert.Equal(t, int32(0), srv.FetchTestRunHitCount())
}

func TestProvisionLocalExecution_ErrorAtUpload(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)

	srv.HandleCreateLoadTest(testProjectID, func(w http.ResponseWriter, _ *http.Request) {
		writeCreateResponse(w)
	})

	srv.HandleStartLocalExecution(testLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		writeSLEResponse(w, srv.PresignedUploadURL())
	})

	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Request has expired"))
	})

	srv.HandleFetchTestRun(testTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	_, err = client.ProvisionLocalExecution(t.Context(), ProvisionParams{
		Name:          "my-test",
		ProjectID:     testProjectID,
		MaxVUs:        50,
		TotalDuration: 630,
		Options:       json.RawMessage(`{"vus":10}`),
		Archive:       newTestArchive(t),
		PollInterval:  time.Microsecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload archive")

	// Poll must not be called.
	assert.Equal(t, int32(0), srv.FetchTestRunHitCount())
}

func TestProvisionLocalExecution_ErrorAtPolling(t *testing.T) {
	t.Parallel()

	srv := provtest.NewServer(t)

	srv.HandleCreateLoadTest(testProjectID, func(w http.ResponseWriter, _ *http.Request) {
		writeCreateResponse(w)
	})

	srv.HandleStartLocalExecution(testLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		writeSLEResponse(w, srv.PresignedUploadURL())
	})

	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv.HandleFetchTestRun(testTestRunID, []v6.TestProgress{
		{
			Status: v6.StatusAborted,
			StatusHistory: []v6.StatusEvent{
				{Status: v6.StatusAborted, Message: "quota exceeded"},
			},
		},
	})

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "0.42.0", 7, 5*time.Second)
	require.NoError(t, err)

	_, err = client.ProvisionLocalExecution(t.Context(), ProvisionParams{
		Name:          "my-test",
		ProjectID:     testProjectID,
		MaxVUs:        50,
		TotalDuration: 630,
		Options:       json.RawMessage(`{"vus":10}`),
		Archive:       newTestArchive(t),
		PollInterval:  time.Microsecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wait for test run ready")
	assert.Contains(t, err.Error(), "quota exceeded")
}
