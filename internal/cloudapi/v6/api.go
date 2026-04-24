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
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"

	"go.k6.io/k6/v2/lib"
)

// StartLocalExecutionRequest is the request body for start_local_execution.
type StartLocalExecutionRequest struct {
	Options       map[string]any `json:"options"`
	MaxVUs        int64          `json:"max_vus"`
	TotalDuration int64          `json:"total_duration"`
	Instances     *int64         `json:"instances,omitempty"`
	ArchiveSize   *int64         `json:"archive_size,omitempty"`
}

// StartLocalExecutionResponse is the response from start_local_execution.
type StartLocalExecutionResponse struct {
	TestRunID             int64         `json:"test_run_id"`
	ArchiveUploadURL      *string       `json:"archive_upload_url"`
	RuntimeConfig         RuntimeConfig `json:"runtime_config"`
	TestRunDetailsPageURL string        `json:"test_run_details_page_url"`
}

// RuntimeConfig holds runtime configuration returned by the provisioning API.
type RuntimeConfig struct {
	Metrics      MetricsRuntimeConfig `json:"metrics"`
	Traces       TracesRuntimeConfig  `json:"traces"`
	Files        FilesRuntimeConfig   `json:"files"`
	Logs         LogsRuntimeConfig    `json:"logs"`
	TestRunToken string               `json:"test_run_token"`
}

// MetricsRuntimeConfig holds metrics runtime configuration.
type MetricsRuntimeConfig struct {
	PushURL               string  `json:"push_url"`
	PushInterval          *string `json:"push_interval"`
	PushConcurrency       *int64  `json:"push_concurrency"`
	AggregationPeriod     *string `json:"aggregation_period"`
	AggregationWaitPeriod *string `json:"aggregation_wait_period"`
	AggregationMinSamples *int64  `json:"aggregation_min_samples"`
	MaxSamplesPerPackage  *int64  `json:"max_samples_per_package"`
}

// TracesRuntimeConfig holds traces runtime configuration.
type TracesRuntimeConfig struct {
	PushURL string `json:"push_url"`
}

// FilesRuntimeConfig holds files runtime configuration.
type FilesRuntimeConfig struct {
	PushURL string `json:"push_url"`
}

// LogsRuntimeConfig holds logs runtime configuration.
type LogsRuntimeConfig struct {
	PushURL string `json:"push_url"`
	TailURL string `json:"tail_url"`
}

// ValidateToken validates the cloud authentication token.
func (c *Client) ValidateToken(ctx context.Context, stackURL string) (_ *k6cloud.AuthenticationResponse, err error) {
	if stackURL == "" {
		return nil, errors.New("stack URL is required to validate token")
	}
	if _, err := url.Parse(stackURL); err != nil {
		return nil, fmt.Errorf("invalid stack URL: %w", err)
	}

	res, hr, err := c.apiClient.AuthorizationAPI.
		Auth(c.authCtx(ctx)).
		XStackUrl(stackURL).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return res, nil
}

// ValidateOptions validates cloud test options.
func (c *Client) ValidateOptions(ctx context.Context, projectID int32, opts lib.Options) (err error) {
	// Round-trip [lib.Options] through JSON so every script option
	// reaches the backend via [k6cloud.Options.AdditionalProperties].
	buf, err := json.Marshal(opts)
	if err != nil {
		return err
	}
	copts := *k6cloud.NewOptions()
	if err := json.Unmarshal(buf, &copts.AdditionalProperties); err != nil {
		return err
	}

	req := k6cloud.NewValidateOptionsRequest(copts)
	if projectID > 0 {
		req.SetProjectId(projectID)
	}
	res, hr, err := c.apiClient.LoadTestsAPI.
		ValidateOptions(c.authCtx(ctx)).
		XStackId(c.stackID).
		ValidateOptionsRequest(req).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return err
	}
	if res == nil {
		return errUnknown
	}

	return nil
}

// UploadTest creates or updates a cloud load test's script.
func (c *Client) UploadTest(
	ctx context.Context, name string, projectID int32, arc *lib.Archive,
) (*k6cloud.LoadTestApiModel, error) {
	lt, err := c.createTest(ctx, name, projectID, arc)
	if err == nil {
		return lt, nil
	}
	var rerr ResponseError
	if !errors.As(err, &rerr) || rerr.Response == nil || rerr.Response.StatusCode != http.StatusConflict {
		return nil, err
	}

	// 409: a test with this name already exists in this project. Look it
	// up by exact-match filter and update its script.
	lt, err = c.findTestByName(ctx, projectID, name)
	if err != nil {
		return nil, err
	}
	if err := c.updateScript(ctx, lt.GetId(), arc); err != nil {
		return nil, err
	}

	return lt, nil
}

// CreateOrFindLoadTest creates a new load test in the project (name only, no script)
// or, on 409 conflict, finds and returns the existing test with the same name.
// Returns the load test ID.
func (c *Client) CreateOrFindLoadTest(ctx context.Context, projectID int32, name string) (int32, error) {
	res, hr, err := c.apiClient.LoadTestsAPI.
		ProjectsLoadTestsCreate(c.authCtx(ctx), projectID).
		XStackId(c.stackID).
		Name(name).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		var rerr ResponseError
		if !errors.As(err, &rerr) || rerr.Response == nil || rerr.Response.StatusCode != http.StatusConflict {
			return 0, err
		}
		lt, ferr := c.findTestByName(ctx, projectID, name)
		if ferr != nil {
			return 0, ferr
		}
		return lt.GetId(), nil
	}
	if res == nil {
		return 0, errUnknown
	}

	return res.GetId(), nil
}

// createTest creates a new cloud load test in the given project.
func (c *Client) createTest(
	ctx context.Context, name string, projectID int32, arc *lib.Archive,
) (_ *k6cloud.LoadTestApiModel, err error) {
	res, hr, err := c.apiClient.LoadTestsAPI.
		ProjectsLoadTestsCreate(c.authCtx(ctx), projectID).
		XStackId(c.stackID).
		Name(name).
		Script(archiveReader(arc)).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return res, nil
}

func (c *Client) findTestByName(
	ctx context.Context, projectID int32, name string,
) (_ *k6cloud.LoadTestApiModel, err error) {
	res, hr, err := c.apiClient.LoadTestsAPI.
		ProjectsLoadTestsRetrieve(c.authCtx(ctx), projectID).
		XStackId(c.stackID).
		Name(name).
		Top(1).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	tests := res.GetValue()
	if len(tests) == 0 {
		return nil, errTestNotExists
	}

	return &tests[0], nil
}

func (c *Client) updateScript(ctx context.Context, testID int32, arc *lib.Archive) (err error) {
	res, err := c.apiClient.LoadTestsAPI.
		LoadTestsScriptUpdate(c.authCtx(ctx), testID).
		XStackId(c.stackID).
		Body(archiveReader(arc)).
		Execute()
	defer closeResponse(res, &err)

	return CheckResponse(res, err)
}

// StartTest starts a cloud load test run.
func (c *Client) StartTest(ctx context.Context, loadTestID int32) (_ *k6cloud.StartLoadTestResponse, err error) {
	var key [8]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, err
	}

	res, hr, err := c.apiClient.LoadTestsAPI.
		LoadTestsStart(c.authCtx(ctx), loadTestID).
		XStackId(c.stackID).
		K6IdempotencyKey(hex.EncodeToString(key[:])).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return res, nil
}

// StopTest aborts a running cloud test run.
func (c *Client) StopTest(ctx context.Context, testRunID int32) (err error) {
	hr, err := c.apiClient.TestRunsAPI.
		TestRunsAbort(c.authCtx(ctx), testRunID).
		XStackId(c.stackID).
		Execute()
	defer closeResponse(hr, &err)

	err = CheckResponse(hr, err)
	var rerr ResponseError
	if errors.As(err, &rerr) && rerr.Response != nil && rerr.Response.StatusCode == http.StatusConflict {
		return nil // Already stopped: swallow the error to keep the caller/TUI clean.
	}

	return err
}

// FetchTest fetches the current progress of a cloud test run.
func (c *Client) FetchTest(ctx context.Context, testRunID int32) (_ *TestProgress, err error) {
	res, hr, err := c.apiClient.TestRunsAPI.
		TestRunsRetrieve(c.authCtx(ctx), testRunID).
		XStackId(c.stackID).
		Execute()
	defer closeResponse(hr, &err)

	if err := CheckResponse(hr, err); err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errUnknown
	}

	return &TestProgress{
		Status:            Status(res.GetStatus()),
		Result:            Result(res.GetResult()),
		EstimatedDuration: res.GetEstimatedDuration(),
		ExecutionDuration: res.GetExecutionDuration(),
		StatusHistory:     FromStatusModel(res.GetStatusHistory()),
	}, nil
}

func (c *Client) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, k6cloud.ContextAccessToken, c.token)
}

func closeResponse(res *http.Response, rerr *error) {
	if res == nil {
		return
	}
	_, _ = io.Copy(io.Discard, res.Body)
	if err := res.Body.Close(); err != nil && *rerr == nil {
		*rerr = err
	}
}

func archiveReader(arc *lib.Archive) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(arc.Write(pw))
	}()
	return pr
}

// StartLocalExecution calls POST /provisioning/v1/load_tests/{id}/start_local_execution.
// It uses Bearer auth (not Token scheme) and includes a K6-Idempotency-Key header.
func (c *Client) StartLocalExecution(
	ctx context.Context,
	loadTestID int32,
	req StartLocalExecutionRequest,
) (*StartLocalExecutionResponse, error) {
	rawURL := fmt.Sprintf("%s/provisioning/v1/load_tests/%d/start_local_execution", c.host, loadTestID)

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	var key [8]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, err
	}
	idempotencyKey := hex.EncodeToString(key[:])

	httpClient := c.apiClient.GetConfig().HTTPClient

	var (
		lastErr  error
		lastResp *http.Response
	)

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
		httpReq.Header.Set("K6-Idempotency-Key", idempotencyKey)

		lastResp, lastErr = httpClient.Do(httpReq)
		if lastErr != nil {
			if attempt < MaxRetries {
				time.Sleep(RetryInterval)
				continue
			}
			break
		}

		if lastResp.StatusCode >= 500 && attempt < MaxRetries {
			_, _ = io.Copy(io.Discard, lastResp.Body)
			_ = lastResp.Body.Close()
			time.Sleep(RetryInterval)
			continue
		}

		break
	}

	if lastErr != nil {
		return nil, lastErr
	}

	defer func() {
		_, _ = io.Copy(io.Discard, lastResp.Body)
		_ = lastResp.Body.Close()
	}()

	if lastResp.StatusCode < 200 || lastResp.StatusCode > 299 {
		return nil, fmt.Errorf(
			"unexpected HTTP error from %s: %d %s",
			rawURL,
			lastResp.StatusCode,
			http.StatusText(lastResp.StatusCode),
		)
	}

	var resp StartLocalExecutionResponse
	if err := json.NewDecoder(lastResp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
