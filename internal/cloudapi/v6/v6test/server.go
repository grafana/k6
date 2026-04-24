// Package v6test provides an HTTP test server that simulates the v6
// cloud API.
//
// It keeps v6-specific encoding (nullable fields, status models, SDK
// types) out of the cmd test files. Cmd tests speak in domain terms
// (v6.TestProgress, v6.Status*, v6.Result*); this server translates to
// v6 wire format. When the API moves to v7, only this package changes.
// The cmd tests stay put.
package v6test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"

	cloudapi "go.k6.io/k6/v2/internal/cloudapi/v6"
)

const (
	defaultLoadTestID int32 = 456
	defaultTestRunID  int32 = 123

	// DefaultTestRunID is the test run ID returned by the test server when
	// no custom TestRunID is provided in Config. Exported for use in tests.
	DefaultTestRunID int32 = defaultTestRunID

	// DefaultLoadTestID is the load test ID returned by the test server.
	// Exported for use in tests that need to reference it explicitly.
	DefaultLoadTestID int32 = defaultLoadTestID
)

// Server is a test HTTP server for the v6 cloud API.
type Server struct {
	*httptest.Server

	cfg Config

	mu                  sync.Mutex
	createLoadTestCalls int // tracks how many times handleCreateLoadTest has been called
}

// Config configures the test server behavior. The zero value is a
// valid configuration that serves a completed, passing test run.
type Config struct {
	// TestRunID is the id reported by the start-test response and used
	// in the test-run routes. Defaults to [defaultTestRunID] when zero.
	TestRunID int32

	// ProgressCallback returns the [cloudapi.TestProgress] reported by
	// each test fetch. When nil, a finished passing run is reported.
	ProgressCallback func() *cloudapi.TestProgress

	// InspectArchive, when set, is invoked with each create-load-test
	// request so tests can inspect the uploaded archive before the
	// server returns its canned response.
	InspectArchive func(*http.Request)

	// ConflictOnCreateLoadTest, when true, causes the first POST to
	// /cloud/v6/projects/{projectID}/load_tests to return HTTP 409.
	// Subsequent calls return HTTP 201 (the normal success path).
	// This simulates the name-conflict fallback to GET by name.
	ConflictOnCreateLoadTest bool

	// ArchiveUploadEnabled, when true, adds a PUT /upload handler and
	// returns a non-nil archive_upload_url in the start_local_execution
	// response pointing to that handler.
	ArchiveUploadEnabled bool

	// InspectRequest, when set, is invoked for every request received by
	// the server. It is called synchronously before the handler returns,
	// so callers must not call server methods from it (to avoid deadlocks).
	InspectRequest func(r *http.Request)
}

// NewServer creates a test server that serves v6 API endpoints.
func NewServer(t *testing.T, cfg Config) *Server {
	t.Helper()

	if cfg.TestRunID == 0 {
		cfg.TestRunID = defaultTestRunID
	}

	s := &Server{cfg: cfg}

	// inspect is a helper that invokes cfg.InspectRequest when set, before
	// delegating to the actual handler. This lets tests capture headers and
	// call sequences without each handler needing to be aware of it.
	inspect := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if s.cfg.InspectRequest != nil {
				s.cfg.InspectRequest(r)
			}
			handler(w, r)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cloud/v6/validate_options", inspect(s.handleValidateOptions))
	mux.HandleFunc("POST /cloud/v6/projects/{projectID}/load_tests", inspect(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.InspectArchive != nil {
			s.cfg.InspectArchive(r)
		}
		s.handleCreateLoadTest(w, r)
	}))
	// GET /cloud/v6/projects/{projectID}/load_tests — fallback search by name on 409.
	mux.HandleFunc("GET /cloud/v6/projects/{projectID}/load_tests", inspect(s.handleGetLoadTestsByName))
	mux.HandleFunc(
		fmt.Sprintf("POST /cloud/v6/load_tests/%d/start", defaultLoadTestID),
		inspect(s.handleStartTest),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST /provisioning/v1/load_tests/%d/start_local_execution", defaultLoadTestID),
		inspect(s.handleStartLocalExecution),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /cloud/v6/test_runs/%d", cfg.TestRunID),
		inspect(s.handleGetTestRun),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST /cloud/v6/test_runs/%d/abort", cfg.TestRunID),
		inspect(s.handleAbortTestRun),
	)
	// Catch-all for the metrics push endpoint returned by handleStartLocalExecution.
	mux.HandleFunc("POST /mock/metrics", inspect(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// v1 test-finished endpoint, used by the legacy cloudapi.Client.TestFinished call
	// when the cloud output's testRunID was set externally (k6 cloud run --local-execution).
	mux.HandleFunc(
		fmt.Sprintf("POST /v1/tests/%d", cfg.TestRunID),
		inspect(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	// v6 notify endpoint for the provisioning path.
	mux.HandleFunc(
		fmt.Sprintf("POST /provisioning/v1/test_runs/%d/notify", cfg.TestRunID),
		inspect(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// S3-style archive upload endpoint, only registered when ArchiveUploadEnabled.
	if cfg.ArchiveUploadEnabled {
		mux.HandleFunc("PUT /upload", inspect(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	}

	srv := httptest.NewServer(mux)
	s.Server = srv
	t.Cleanup(srv.Close)

	return s
}

func (s *Server) handleValidateOptions(w http.ResponseWriter, _ *http.Request) {
	vuh, zero := float32(0.5), float32(0)
	res := k6cloud.NewValidateOptionsResponse(
		vuh,
		*k6cloud.NewCostBreakdownApiModel(
			k6cloud.ProtocolVuh{Float32: &vuh},
			k6cloud.BrowserVuh{Float32: &zero},
			k6cloud.BaseTotalVuh{Float32: &vuh},
			k6cloud.ReductionRate{Float32: &zero},
			map[string]k6cloud.ReductionRateBreakdownValue{},
		),
	)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleCreateLoadTest(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.createLoadTestCalls++
	callNum := s.createLoadTestCalls
	s.mu.Unlock()

	// When ConflictOnCreateLoadTest is set, return 409 on the first call to
	// simulate a name conflict. The client should fall back to GET by name.
	// The response body must match ResponseError ({"error": {...}}) so that
	// CheckResponse can deserialize it into a ResponseError and the caller
	// can detect the 409 by checking rerr.Response.StatusCode.
	if s.cfg.ConflictOnCreateLoadTest && callNum == 1 {
		// The response body must include both "message" and "code" fields
		// because ErrorApiModel.UnmarshalJSON validates both are present.
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": map[string]any{
				"message": "a load test with this name already exists in this project",
				"code":    "CONFLICT",
			},
		})
		return
	}

	res := k6cloud.NewLoadTestApiModelWithDefaults()
	res.SetId(defaultLoadTestID)
	writeJSON(w, http.StatusCreated, res)
}

// handleGetLoadTestsByName handles the GET fallback used when POST returns 409.
// It returns a minimal load test list containing one matching load test.
// The response shape matches [k6cloud.LoadTestListResponse]: {"value": [...]}.
func (s *Server) handleGetLoadTestsByName(w http.ResponseWriter, _ *http.Request) {
	lt := k6cloud.NewLoadTestApiModelWithDefaults()
	lt.SetId(defaultLoadTestID)

	resp := map[string]any{
		"value": []any{lt},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStartTest(w http.ResponseWriter, _ *http.Request) {
	res := k6cloud.NewStartLoadTestResponseWithDefaults()
	res.SetId(s.cfg.TestRunID)
	res.SetTestRunDetailsPageUrl(fmt.Sprintf("https://stack.grafana.com/a/k6-app/runs/%d", s.cfg.TestRunID))
	// SDK decoder requires these keys on the other end even when empty.
	res.SetDistribution([]k6cloud.DistributionZoneApiModel{})
	res.SetResultDetails(map[string]any{})
	res.SetOptions(map[string]any{})
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStartLocalExecution(w http.ResponseWriter, _ *http.Request) {
	// Use the mock server URL for the metrics push endpoint so that tests
	// do not accidentally attempt real network connections.
	metricsURL := fmt.Sprintf("%s/mock/metrics", s.URL)

	// archive_upload_url is nil by default. When ArchiveUploadEnabled, return
	// a URL pointing to the PUT /upload handler on this server.
	var archiveUploadURL any
	if s.cfg.ArchiveUploadEnabled {
		archiveUploadURL = fmt.Sprintf("%s/upload", s.URL)
	}

	resp := map[string]any{
		"test_run_id":               s.cfg.TestRunID,
		"archive_upload_url":        archiveUploadURL,
		"test_run_details_page_url": fmt.Sprintf("https://stack.grafana.com/a/k6-app/runs/%d", s.cfg.TestRunID),
		"runtime_config": map[string]any{
			"test_run_token": "mock-test-run-token",
			"metrics": map[string]any{
				"push_url": metricsURL,
			},
			"traces": map[string]any{"push_url": ""},
			"files":  map[string]any{"push_url": ""},
			"logs":   map[string]any{"push_url": "", "tail_url": ""},
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetTestRun(w http.ResponseWriter, _ *http.Request) {
	tp := &cloudapi.TestProgress{
		Status:            cloudapi.StatusCompleted,
		Result:            cloudapi.ResultPassed,
		EstimatedDuration: 60,
		ExecutionDuration: 60,
	}
	if s.cfg.ProgressCallback != nil {
		tp = s.cfg.ProgressCallback()
	}

	res := k6cloud.NewTestRunApiModelWithDefaults()
	res.SetStatus(tp.Status.String())
	res.SetResult(tp.Result.String())
	res.SetEstimatedDuration(tp.EstimatedDuration)
	res.SetExecutionDuration(tp.ExecutionDuration)
	res.SetStatusHistory(cloudapi.ToStatusModel(tp.StatusHistory))
	// SDK decoder requires these keys on the other end even when empty.
	res.SetDistribution([]k6cloud.DistributionZoneApiModel{})
	res.SetResultDetails(map[string]any{})
	res.SetOptions(map[string]any{})
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAbortTestRun(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// ResultNone reports a test still in progress (no result yet).
const ResultNone cloudapi.Result = ""

// Progress returns a progress callback that reports the given status and result.
func Progress(status cloudapi.Status, result cloudapi.Result) func() *cloudapi.TestProgress {
	return func() *cloudapi.TestProgress {
		return &cloudapi.TestProgress{Status: status, Result: result}
	}
}

// AbortedByUserProgress returns a progress callback that reports the test as aborted.
func AbortedByUserProgress(email string) func() *cloudapi.TestProgress {
	return func() *cloudapi.TestProgress {
		return &cloudapi.TestProgress{
			Status: cloudapi.StatusAborted,
			Result: cloudapi.ResultError,
			StatusHistory: []cloudapi.StatusEvent{
				{Status: cloudapi.StatusAborted, ByUser: email},
			},
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(fmt.Errorf("v6test: encoding JSON: %w", err))
	}
}
