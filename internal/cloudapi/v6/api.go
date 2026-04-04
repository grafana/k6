package cloudapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"go.k6.io/k6/lib"
)

// V6 test run statuses per the OpenAPI spec (StatusApiModel).
const (
	StatusCreated           = "created"
	StatusQueued            = "queued"
	StatusInitializing      = "initializing"
	StatusRunning           = "running"
	StatusProcessingMetrics = "processing_metrics"
	StatusCompleted         = "completed"
	StatusAborted           = "aborted"

	ResultFailed = "failed"
	ResultError  = "error"
)

// FormatStatus returns a human-readable display string for a v6 status.
// Unknown statuses pass through as-is.
func FormatStatus(status string) string {
	switch status {
	case StatusCreated:
		return "Created"
	case StatusQueued:
		return "Queued"
	case StatusInitializing:
		return "Initializing"
	case StatusRunning:
		return "Running"
	case StatusProcessingMetrics:
		return "Processing Metrics"
	case StatusCompleted:
		return "Completed"
	case StatusAborted:
		return "Aborted"
	default:
		return status
	}
}

// TestRunProgress maps a subset of the v6 test run response to track
// execution progress.
type TestRunProgress struct {
	Status            string
	Result            string
	EstimatedDuration int32
	ExecutionDuration int32
}

// IsTerminal reports whether the test run status is a terminal state.
func (p TestRunProgress) IsTerminal() bool {
	switch p.Status {
	case StatusCompleted, StatusAborted:
		return true
	default:
		return false
	}
}

// ValidateToken calls the endpoint to validate the Client's token and returns the result.
func (c *Client) ValidateToken(ctx context.Context, stackURL string) (_ *k6cloud.AuthenticationResponse, err error) {
	if stackURL == "" {
		return nil, errors.New("stack URL is required to validate token")
	}
	if _, err := url.Parse(stackURL); err != nil {
		return nil, fmt.Errorf("invalid stack URL: %w", err)
	}

	var result *k6cloud.AuthenticationResponse
	err = c.retryDo(ctx, func() (bool, error) {
		resp, httpRes, rerr := c.apiClient.AuthorizationAPI.
			Auth(c.authCtx(ctx)).
			XStackUrl(stackURL).
			Execute()
		defer closeResponse(httpRes, &rerr)
		if err := CheckResponse(httpRes, rerr); err != nil {
			return shouldRetry(httpRes, rerr), fmt.Errorf("validating token: %w", err)
		}
		result = resp
		return false, nil
	})
	return result, err
}

// ValidateOptions sends the provided options to the cloud for validation.
func (c *Client) ValidateOptions(ctx context.Context, options lib.Options) error {
	raw, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("marshaling options: %w", err)
	}
	var opts k6cloud.Options
	if err := json.Unmarshal(raw, &opts); err != nil {
		return fmt.Errorf("unmarshaling options: %w", err)
	}

	validateOptions := k6cloud.NewValidateOptionsRequest(opts)
	validateOptions.ProjectId = *k6cloud.NewNullableInt32(&c.projectID)

	return c.retryDo(ctx, func() (bool, error) {
		_, httpRes, rerr := c.apiClient.LoadTestsAPI.
			ValidateOptions(c.authCtx(ctx)).
			ValidateOptionsRequest(validateOptions).
			XStackId(c.stackID).
			Execute()
		defer closeResponse(httpRes, &rerr)
		return shouldRetry(httpRes, rerr), CheckResponse(httpRes, rerr)
	})
}

// CreateCloudTest creates a new cloud test with the provided name and script archive.
func (c *Client) CreateCloudTest(
	ctx context.Context, name string, arcData []byte,
) (*k6cloud.LoadTestApiModel, error) {
	var result *k6cloud.LoadTestApiModel
	err := c.retryDo(ctx, func() (bool, error) {
		loadTest, res, rerr := c.apiClient.LoadTestsAPI.
			ProjectsLoadTestsCreate(c.authCtx(ctx), c.projectID).
			Name(name).
			Script(io.NopCloser(bytes.NewReader(arcData))).
			XStackId(c.stackID).
			Execute()
		defer closeResponse(res, &rerr)
		if err := CheckResponse(res, rerr); err != nil {
			return shouldRetry(res, rerr), fmt.Errorf("creating cloud test: %w", err)
		}
		result = loadTest
		return false, nil
	})
	return result, err
}

// FetchCloudTestByName retrieves a cloud test by its name within the specified project.
// The API's name parameter is an exact-match filter.
func (c *Client) FetchCloudTestByName(
	ctx context.Context, name string,
) (*k6cloud.LoadTestApiModel, error) {
	var result *k6cloud.LoadTestApiModel
	err := c.retryDo(ctx, func() (bool, error) {
		loadTests, res, rerr := c.apiClient.LoadTestsAPI.
			ProjectsLoadTestsRetrieve(c.authCtx(ctx), c.projectID).
			XStackId(c.stackID).
			Name(name).
			Execute()
		defer closeResponse(res, &rerr)
		if err := CheckResponse(res, rerr); err != nil {
			return shouldRetry(res, rerr), fmt.Errorf("fetching cloud test by name: %w", err)
		}
		idx := slices.IndexFunc(loadTests.Value, func(t k6cloud.LoadTestApiModel) bool {
			return t.Name == name
		})
		if idx < 0 {
			return false, fmt.Errorf("load test %q not found in project", name)
		}
		result = &loadTests.Value[idx]
		return false, nil
	})
	return result, err
}

// CreateOrUpdateCloudTest creates a new cloud test or updates an existing one
// if a test with the same name already exists (409 Conflict).
func (c *Client) CreateOrUpdateCloudTest(
	ctx context.Context, name string, arc *lib.Archive,
) (*k6cloud.LoadTestApiModel, error) {
	var buf bytes.Buffer
	if err := arc.Write(&buf); err != nil {
		return nil, fmt.Errorf("writing archive: %w", err)
	}
	arcData := buf.Bytes()

	test, err := c.CreateCloudTest(ctx, name, arcData)
	if err != nil {
		var rErr ResponseError
		if !errors.As(err, &rErr) || rErr.Response.StatusCode != http.StatusConflict {
			return nil, err
		}

		test, err = c.FetchCloudTestByName(ctx, name)
		if err != nil {
			return nil, err
		}

		updateErr := c.retryDo(ctx, func() (bool, error) {
			res, rerr := c.apiClient.LoadTestsAPI.
				LoadTestsScriptUpdate(c.authCtx(ctx), test.Id).
				Body(io.NopCloser(bytes.NewReader(arcData))).
				XStackId(c.stackID).
				Execute()
			defer closeResponse(res, &rerr)
			if err := CheckResponse(res, rerr); err != nil {
				return shouldRetry(res, rerr), fmt.Errorf("updating cloud test script: %w", err)
			}
			return false, nil
		})
		if updateErr != nil {
			return nil, updateErr
		}
	}

	return test, nil
}

// StartCloudTestRun starts a new cloud test run for a given test.
func (c *Client) StartCloudTestRun(
	ctx context.Context, loadTestID int32,
) (*k6cloud.StartLoadTestResponse, error) {
	// Generate the key once so retries reuse it — the backend deduplicates
	// by this key, so a fresh key per attempt would defeat idempotency.
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	idempotencyKey := hex.EncodeToString(b)

	var result *k6cloud.StartLoadTestResponse
	err := c.retryDo(ctx, func() (bool, error) {
		loadTestRun, res, rerr := c.apiClient.LoadTestsAPI.
			LoadTestsStart(c.authCtx(ctx), loadTestID).
			XStackId(c.stackID).
			K6IdempotencyKey(idempotencyKey).
			Execute()
		defer closeResponse(res, &rerr)
		if err := CheckResponse(res, rerr); err != nil {
			return shouldRetry(res, rerr), fmt.Errorf("starting cloud test run: %w", err)
		}
		result = loadTestRun
		return false, nil
	})
	return result, err
}

// StopCloudTestRun tells the cloud to abort the test run.
func (c *Client) StopCloudTestRun(ctx context.Context, testRunID int64) error {
	testRunID32, err := toInt32(testRunID)
	if err != nil {
		return fmt.Errorf("converting test run ID: %w", err)
	}

	return c.retryDo(ctx, func() (bool, error) {
		res, rerr := c.apiClient.TestRunsAPI.
			TestRunsAbort(c.authCtx(ctx), testRunID32).
			XStackId(c.stackID).
			Execute()
		defer closeResponse(res, &rerr)
		if err := CheckResponse(res, rerr); err != nil {
			return shouldRetry(res, rerr), fmt.Errorf("stopping cloud test run: %w", err)
		}
		return false, nil
	})
}

// FetchTestRun calls GET /cloud/v6/test_runs/{id} and returns the test run progress.
// Transient errors are retried via retryDo.
func (c *Client) FetchTestRun(ctx context.Context, testRunID int64) (*TestRunProgress, error) {
	testRunID32, err := toInt32(testRunID)
	if err != nil {
		return nil, fmt.Errorf("converting test run ID: %w", err)
	}

	var progress *TestRunProgress
	err = c.retryDo(ctx, func() (bool, error) {
		resp, res, rerr := c.apiClient.TestRunsAPI.
			TestRunsRetrieve(c.authCtx(ctx), testRunID32).
			XStackId(c.stackID).
			Execute()
		defer closeResponse(res, &rerr)
		if err := CheckResponse(res, rerr); err != nil {
			return shouldRetry(res, rerr), err
		}
		progress = &TestRunProgress{
			Status:            resp.Status,
			Result:            resp.GetResult(),
			EstimatedDuration: resp.GetEstimatedDuration(),
			ExecutionDuration: resp.ExecutionDuration,
		}
		return false, nil
	})
	return progress, err
}
