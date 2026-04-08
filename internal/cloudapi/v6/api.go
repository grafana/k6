package cloudapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"go.k6.io/k6/lib"
)

// ValidateToken calls the endpoint to validate the Client's token and returns the result.
func (c *Client) ValidateToken(ctx context.Context, stackURL string) (_ *k6cloud.AuthenticationResponse, err error) {
	if stackURL == "" {
		return nil, errors.New("stack URL is required to validate token")
	}

	if _, err := url.Parse(stackURL); err != nil {
		return nil, fmt.Errorf("invalid stack URL: %w", err)
	}

	resp, res, rerr := c.apiClient.AuthorizationAPI.
		Auth(c.authCtx(ctx)).
		XStackUrl(stackURL).
		Execute()
	defer closeResponse(res, &err)

	return resp, checkRequest(res, rerr, "validate token")
}

// ValidateOptions sends the provided options to the cloud for validation.
func (c *Client) ValidateOptions(ctx context.Context, projectID int64, options lib.Options) (err error) {
	validateOptions := k6cloud.NewValidateOptionsRequest(mapOptions(options))
	if projectID > 0 {
		pid, err := checkInt32("project ID", projectID)
		if err != nil {
			return fmt.Errorf("checking project ID: %w", err)
		}
		validateOptions.SetProjectId(pid)
	}

	stackID, err := checkInt32("stack ID", c.stackID)
	if err != nil {
		return fmt.Errorf("checking stack ID: %w", err)
	}

	_, res, rerr := c.apiClient.LoadTestsAPI.
		ValidateOptions(c.authCtx(ctx)).
		ValidateOptionsRequest(validateOptions).
		XStackId(stackID).
		Execute()
	defer closeResponse(res, &err)

	return checkRequest(res, rerr, "validate options")
}

func mapOptions(options lib.Options) k6cloud.Options {
	opts := *k6cloud.NewOptions()
	opts.AdditionalProperties = make(map[string]any)

	if options.VUs.Valid {
		opts.AdditionalProperties["vus"] = options.VUs.Int64
	}
	if options.Duration.Valid {
		opts.AdditionalProperties["duration"] = options.Duration.String()
	}
	if len(options.Stages) > 0 {
		opts.AdditionalProperties["stages"] = options.Stages
	}
	if len(options.Scenarios) > 0 {
		opts.AdditionalProperties["scenarios"] = options.Scenarios
	}

	return opts
}

// TestRun is the subset of the start response the remote-run command needs.
type TestRun struct {
	ID        int64
	WebAppURL string
}

// Test run status and result values used by the remote-run command path.
const (
	StatusCreated            = "created"
	StatusInitializing       = "initializing"
	StatusRunning            = "running"
	StatusProcessingMetrics  = "processing_metrics"
	StatusCompleted          = "completed"
	StatusTimedOut           = "timed_out"
	StatusAbortedUser        = "aborted_user"
	StatusAbortedSystem      = "aborted_system"
	StatusAbortedScriptError = "aborted_script_error"
	StatusAbortedThreshold   = "aborted_threshold"
	StatusAbortedLimit       = "aborted_limit"

	ResultPassed = "passed"
	ResultFailed = "failed"
	ResultError  = "error"
)

// TestRunProgress is the subset of the polling response the remote-run command needs.
type TestRunProgress struct {
	Status            string
	Result            string
	ExecutionDuration int32
	EstimatedDuration int32
}

// IsFinished reports whether the remote run reached a terminal state.
func (p TestRunProgress) IsFinished() bool {
	switch p.Status {
	case StatusCompleted, StatusTimedOut:
		return true
	default:
		return p.IsAborted()
	}
}

// IsRunning reports whether the remote run is actively executing.
func (p TestRunProgress) IsRunning() bool {
	return p.Status == StatusRunning
}

// IsAborted reports whether the remote run ended in an aborted state.
func (p TestRunProgress) IsAborted() bool {
	switch p.Status {
	case StatusAbortedUser, StatusAbortedSystem, StatusAbortedScriptError, StatusAbortedThreshold, StatusAbortedLimit:
		return true
	default:
		return false
	}
}

// Progress reports the run progress as a value between 0 and 1.
func (p TestRunProgress) Progress() float64 {
	if p.EstimatedDuration <= 0 {
		return 0
	}

	return min(1.0, float64(p.ExecutionDuration)/float64(p.EstimatedDuration))
}

// FormatStatus converts a v6 test-run status into CLI display text.
func FormatStatus(status string) string {
	switch status {
	case StatusCreated:
		return "Created"
	case StatusInitializing:
		return "Initializing"
	case StatusRunning:
		return "Running"
	case StatusProcessingMetrics:
		return "Processing Metrics"
	case StatusCompleted:
		return "Completed"
	case StatusTimedOut:
		return "Timed Out"
	case StatusAbortedUser, StatusAbortedSystem, StatusAbortedScriptError, StatusAbortedThreshold, StatusAbortedLimit:
		return "Aborted"
	default:
		return status
	}
}

// StartTest creates a new cloud test and starts a remote run for it.
func (c *Client) StartTest(ctx context.Context, name string, projectID int64,
	arc *lib.Archive,
) (_ *TestRun, err error) {
	pid, err := checkInt32("project ID", projectID)
	if err != nil {
		return nil, fmt.Errorf("checking project ID: %w", err)
	}
	stackID, err := checkInt32("stack ID", c.stackID)
	if err != nil {
		return nil, fmt.Errorf("checking stack ID: %w", err)
	}

	loadTest, err := c.createOrUpdateCloudTest(ctx, pid, stackID, name, arc)
	if err != nil {
		return nil, fmt.Errorf("creating cloud test: %w", err)
	}

	return c.startCloudTestRun(ctx, stackID, loadTest.Id)
}

// UploadTest creates a new cloud test or updates the existing one on name conflict.
func (c *Client) UploadTest(ctx context.Context, name string, projectID int64, arc *lib.Archive) (int64, error) {
	pid, err := checkInt32("project ID", projectID)
	if err != nil {
		return 0, fmt.Errorf("checking project ID: %w", err)
	}
	stackID, err := checkInt32("stack ID", c.stackID)
	if err != nil {
		return 0, fmt.Errorf("checking stack ID: %w", err)
	}

	loadTest, err := c.createOrUpdateCloudTest(ctx, pid, stackID, name, arc)
	if err != nil {
		return 0, fmt.Errorf("uploading cloud test: %w", err)
	}

	return int64(loadTest.Id), nil
}

func (c *Client) createOrUpdateCloudTest(ctx context.Context, projectID int32, stackID int32, name string,
	arc *lib.Archive,
) (_ *k6cloud.LoadTestApiModel, err error) {
	loadTest, err := c.createCloudTest(ctx, projectID, stackID, name, arc)
	if err == nil {
		return loadTest, nil
	}

	var rerr ResponseError
	if !errors.As(err, &rerr) || rerr.Response.StatusCode != http.StatusConflict {
		return nil, fmt.Errorf("creating cloud test: %w", err)
	}

	loadTest, err = c.fetchCloudTestByName(ctx, projectID, stackID, name)
	if err != nil {
		return nil, fmt.Errorf("fetching cloud test: %w", err)
	}
	if err = c.updateCloudTest(ctx, stackID, loadTest.Id, arc); err != nil {
		return nil, fmt.Errorf("updating cloud test: %w", err)
	}

	return loadTest, nil
}

func (c *Client) createCloudTest(ctx context.Context, projectID int32, stackID int32, name string,
	arc *lib.Archive,
) (_ *k6cloud.LoadTestApiModel, err error) {
	loadTest, res, rerr := c.apiClient.LoadTestsAPI.
		ProjectsLoadTestsCreate(c.authCtx(ctx), projectID).
		Name(name).
		Script(archiveReader(arc)).
		XStackId(stackID).
		Execute()
	defer closeResponse(res, &err)

	if err = checkRequest(res, rerr, "creating cloud test"); err != nil {
		return nil, err
	}
	if loadTest == nil {
		return nil, errors.New("empty load test response")
	}

	return loadTest, nil
}

func (c *Client) startCloudTestRun(ctx context.Context, stackID int32, loadTestID int32) (_ *TestRun, err error) {
	key := make([]byte, 8)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating idempotency key: %w", err)
	}

	loadTestRun, res, rerr := c.apiClient.LoadTestsAPI.
		LoadTestsStart(c.authCtx(ctx), loadTestID).
		XStackId(stackID).
		K6IdempotencyKey(hex.EncodeToString(key)).
		Execute()
	defer closeResponse(res, &err)

	if err = checkRequest(res, rerr, "starting cloud test run"); err != nil {
		return nil, err
	}
	if loadTestRun == nil {
		return nil, errors.New("empty start cloud test run response")
	}

	return &TestRun{
		ID:        int64(loadTestRun.Id),
		WebAppURL: loadTestRun.TestRunDetailsPageUrl,
	}, nil
}

func (c *Client) fetchCloudTestByName(
	ctx context.Context,
	projectID int32,
	stackID int32,
	name string,
) (_ *k6cloud.LoadTestApiModel, err error) {
	resp, res, rerr := c.apiClient.LoadTestsAPI.
		LoadTestsList(c.authCtx(ctx)).
		XStackId(stackID).
		Name(name).
		Execute()
	defer closeResponse(res, &err)

	if err = checkRequest(res, rerr, "fetching cloud test"); err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("empty load test list response")
	}

	idx := slices.IndexFunc(resp.Value, func(loadTest k6cloud.LoadTestApiModel) bool {
		return loadTest.ProjectId == projectID && loadTest.Name == name
	})
	if idx < 0 {
		return nil, fmt.Errorf("cloud test %q not found", name)
	}

	return &resp.Value[idx], nil
}

func (c *Client) updateCloudTest(ctx context.Context, stackID int32, loadTestID int32, arc *lib.Archive) (err error) {
	res, rerr := c.apiClient.LoadTestsAPI.
		LoadTestsScriptUpdate(c.authCtx(ctx), loadTestID).
		XStackId(stackID).
		Body(archiveReader(arc)).
		Execute()
	defer closeResponse(res, &err)

	return checkRequest(res, rerr, "updating cloud test")
}

func archiveReader(arc *lib.Archive) io.ReadCloser {
	pr, pw := io.Pipe()

	go func() {
		pw.CloseWithError(arc.Write(pw))
	}()

	return pr
}

// FetchTest retrieves the current remote-run status for the CLI wait loop.
func (c *Client) FetchTest(ctx context.Context, testRunID int64) (_ *TestRunProgress, err error) {
	trid, err := checkInt32("test run ID", testRunID)
	if err != nil {
		return nil, fmt.Errorf("checking test run ID: %w", err)
	}
	stackID, err := checkInt32("stack ID", c.stackID)
	if err != nil {
		return nil, fmt.Errorf("checking stack ID: %w", err)
	}

	resp, res, rerr := c.apiClient.TestRunsAPI.
		TestRunsRetrieve(c.authCtx(ctx), trid).
		XStackId(stackID).
		Execute()
	defer closeResponse(res, &err)

	if err = checkRequest(res, rerr, "fetching test run"); err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("empty test run response")
	}

	return &TestRunProgress{
		Status:            resp.Status,
		Result:            resp.GetResult(),
		ExecutionDuration: resp.ExecutionDuration,
		EstimatedDuration: resp.GetEstimatedDuration(),
	}, nil
}

// StopTest aborts a remote run.
func (c *Client) StopTest(ctx context.Context, testRunID int64) (err error) {
	trid, err := checkInt32("test run ID", testRunID)
	if err != nil {
		return fmt.Errorf("checking test run ID: %w", err)
	}
	stackID, err := checkInt32("stack ID", c.stackID)
	if err != nil {
		return fmt.Errorf("checking stack ID: %w", err)
	}

	res, rerr := c.apiClient.TestRunsAPI.
		TestRunsAbort(c.authCtx(ctx), trid).
		XStackId(stackID).
		Execute()
	defer closeResponse(res, &err)

	return checkRequest(res, rerr, "aborting cloud test run")
}
