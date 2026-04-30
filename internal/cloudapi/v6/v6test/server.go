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
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"

	cloudapi "go.k6.io/k6/v2/internal/cloudapi/v6"
)

const (
	defaultLoadTestID int32 = 456
	defaultTestRunID  int32 = 123
)

// Server is a test HTTP server for the v6 cloud API.
type Server struct {
	*httptest.Server

	cfg Config
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

	// Projects is the project list returned by the projects endpoint.
	Projects []cloudapi.Project
}

// NewServer creates a test server that serves v6 API endpoints.
func NewServer(t *testing.T, cfg Config) *Server {
	t.Helper()

	if cfg.TestRunID == 0 {
		cfg.TestRunID = defaultTestRunID
	}

	s := &Server{cfg: cfg}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /cloud/v6/projects", s.handleListProjects)
	mux.HandleFunc("POST /cloud/v6/validate_options", s.handleValidateOptions)
	mux.HandleFunc("POST /cloud/v6/projects/{projectID}/load_tests", func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.InspectArchive != nil {
			s.cfg.InspectArchive(r)
		}
		s.handleCreateLoadTest(w, r)
	})
	mux.HandleFunc(
		fmt.Sprintf("POST /cloud/v6/load_tests/%d/start", defaultLoadTestID),
		s.handleStartTest,
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /cloud/v6/test_runs/%d", cfg.TestRunID),
		s.handleGetTestRun,
	)
	mux.HandleFunc(
		fmt.Sprintf("POST /cloud/v6/test_runs/%d/abort", cfg.TestRunID),
		s.handleAbortTestRun,
	)

	srv := httptest.NewServer(mux)
	s.Server = srv
	t.Cleanup(srv.Close)

	return s
}

func (s *Server) handleListProjects(w http.ResponseWriter, _ *http.Request) {
	projects := make([]k6cloud.ProjectApiModel, len(s.cfg.Projects))
	now := time.Unix(0, 0).UTC()
	for i, project := range s.cfg.Projects {
		projects[i] = *k6cloud.NewProjectApiModel(
			project.ID,
			project.Name,
			project.IsDefault,
			*k6cloud.NewNullableString(nil),
			now,
			now,
		)
	}
	res := k6cloud.NewProjectListResponse(projects)
	res.SetCount(int32(len(projects)))
	writeJSON(w, http.StatusOK, res)
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
	res := k6cloud.NewLoadTestApiModelWithDefaults()
	res.SetId(defaultLoadTestID)
	writeJSON(w, http.StatusCreated, res)
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
