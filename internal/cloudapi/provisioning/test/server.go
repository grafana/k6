// Package provtest provides an HTTP test server that simulates the
// provisioning API. Routes are registered by individual specs as they
// add API methods; this skeleton starts with an empty mux.
package provtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"

	cloudapi "go.k6.io/k6/v2/internal/cloudapi/v6"
)

const (
	// DefaultLoadTestID is the load test ID used by default handlers.
	DefaultLoadTestID int32 = 456
	// DefaultTestRunID is the test run ID returned by default handlers.
	DefaultTestRunID int32 = 123
	// PresignedUploadPath is the default path used for the presigned archive upload stub.
	PresignedUploadPath = "/upload/archive.tar"
)

// Server is a test HTTP server for the provisioning API.
type Server struct {
	*httptest.Server
	Mux *http.ServeMux

	fetchTestRunHits *atomic.Int32
}

// NewServer creates and starts a test server with an empty route table.
// The server is automatically closed when the test finishes.
func NewServer(t *testing.T) *Server {
	t.Helper()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &Server{
		Server: srv,
		Mux:    mux,
	}
}

// HandleStartLocalExecution registers a handler for
// POST /provisioning/v1/load_tests/{loadTestID}/start_local_execution.
// If handler is nil, a default handler returning a successful response is used.
func (s *Server) HandleStartLocalExecution(loadTestID int32, handler http.HandlerFunc) {
	if handler == nil {
		handler = func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, DefaultStartLocalExecutionResponse())
		}
	}
	s.Mux.HandleFunc(
		fmt.Sprintf("POST /provisioning/v1/load_tests/%d/start_local_execution", loadTestID),
		handler,
	)
}

// DefaultStartLocalExecutionResponse returns a fully-populated
// StartLocalExecutionTestResponse suitable for test assertions.
func DefaultStartLocalExecutionResponse() *k6cloud.StartLocalExecutionTestResponse {
	metrics := k6cloud.NewMetricsRuntimeConfig(
		"https://ingest.k6.io/v1/metrics",
		*k6cloud.NewNullableString(strPtr("2s")),
		*k6cloud.NewNullableInt32(int32Ptr(5)),
		*k6cloud.NewNullableString(strPtr("3s")),
		*k6cloud.NewNullableString(strPtr("1s")),
		*k6cloud.NewNullableInt32(int32Ptr(50)),
		*k6cloud.NewNullableInt32(int32Ptr(2000)),
	)

	secrets := k6cloud.NewSecretsRuntimeConfig(
		"https://api.k6.io/provisioning/v1/test_runs/123/decrypt_secret?name={key}",
		"plaintext",
	)

	traces := k6cloud.NewTracesRuntimeConfig(
		"https://traces.k6.io",
		map[string]string{},
		"http",
	)
	files := k6cloud.NewFilesRuntimeConfig(
		"https://files.k6.io",
		"/screenshots",
		map[string]string{},
		"POST",
	)
	logs := k6cloud.NewLogsRuntimeConfig(
		"https://logs.k6.io",
		"https://logs.k6.io/tail",
		900,
		"3s",
		"info",
		10000,
		[]string{"lz", "level"},
	)

	rc := k6cloud.NewRuntimeConfig(
		*metrics,
		*traces,
		*files,
		*logs,
		*secrets,
		"test-run-token-abc",
	)

	uploadURL := "https://s3.amazonaws.com/bucket/archive.tar?presigned=1"
	return k6cloud.NewStartLocalExecutionTestResponse(
		DefaultTestRunID,
		*rc,
		*k6cloud.NewNullableString(&uploadURL),
		fmt.Sprintf("https://app.k6.io/runs/%d", DefaultTestRunID),
	)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(fmt.Errorf("provtest: encoding JSON: %w", err))
	}
}

// HandlePresignedUpload registers a handler for PUT requests at the given path,
// simulating a presigned S3 upload endpoint.
func (s *Server) HandlePresignedUpload(path string, handler http.HandlerFunc) {
	s.Mux.HandleFunc("PUT "+path, handler)
}

// PresignedUploadURL returns the full URL for the presigned upload stub
// using the default PresignedUploadPath.
func (s *Server) PresignedUploadURL() string {
	return s.URL + PresignedUploadPath
}

// HandleFetchTestRun registers a handler for
// GET /cloud/v6/test_runs/{testRunID}. The handler returns successive
// TestProgress values from the provided sequence. Once the sequence is
// exhausted, the last element is repeated.
//
// It also tracks the number of times the handler was called; use
// FetchTestRunHitCount to query it.
func (s *Server) HandleFetchTestRun(testRunID int32, sequence []cloudapi.TestProgress) {
	var idx atomic.Int32
	var hits atomic.Int32

	s.fetchTestRunHits = &hits

	s.Mux.HandleFunc(
		fmt.Sprintf("GET /cloud/v6/test_runs/%d", testRunID),
		func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)

			i := int(idx.Load())
			if i < len(sequence)-1 {
				idx.Add(1)
			}
			tp := sequence[i]

			res := k6cloud.NewTestRunApiModelWithDefaults()
			res.SetStatus(tp.Status.String())
			res.SetResult(tp.Result.String())
			res.SetEstimatedDuration(tp.EstimatedDuration)
			res.SetExecutionDuration(tp.ExecutionDuration)
			res.SetStatusHistory(cloudapi.ToStatusModel(tp.StatusHistory))
			res.SetDistribution([]k6cloud.DistributionZoneApiModel{})
			res.SetResultDetails(map[string]any{})
			res.SetOptions(map[string]any{})
			writeJSON(w, http.StatusOK, res)
		},
	)
}

// FetchTestRunHitCount returns the number of times the FetchTestRun
// handler was called. Returns 0 if HandleFetchTestRun was never called.
func (s *Server) FetchTestRunHitCount() int32 {
	if s.fetchTestRunHits == nil {
		return 0
	}
	return s.fetchTestRunHits.Load()
}

// HandleNotify registers a handler for
// POST /provisioning/v1/test_runs/{testRunID}/notify.
func (s *Server) HandleNotify(testRunID int32, handler http.HandlerFunc) {
	s.Mux.HandleFunc(
		fmt.Sprintf("POST /provisioning/v1/test_runs/%d/notify", testRunID),
		handler,
	)
}

func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }
