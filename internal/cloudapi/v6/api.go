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

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"go.k6.io/k6/lib"
)

// CreateCloudTest creates a new cloud test with the provided name and script archive.
func (c *Client) CreateCloudTest(
	name string, projectID int64, arc *lib.Archive,
) (lt *k6cloud.LoadTestApiModel, err error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)
	var buf bytes.Buffer
	if err := arc.Write(&buf); err != nil {
		return nil, err
	}

	projectID32, err := toInt32(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	reqCreate := c.apiClient.LoadTestsAPI.ProjectsLoadTestsCreate(ctx, projectID32).
		Name(name).
		Script(io.NopCloser(bytes.NewReader(buf.Bytes()))).
		XStackId(c.stackID)

	loadTest, httpRes, rerr := reqCreate.Execute()
	defer closeResponse(httpRes, &err)
	if err := CheckResponse(httpRes, rerr); err != nil {
		return nil, err
	}

	return loadTest, nil
}

// updateCloudTest updates an existing cloud test with the provided script archive.
func (c *Client) updateCloudTest(testID int32, arc *lib.Archive) (err error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)
	var buf bytes.Buffer
	if err := arc.Write(&buf); err != nil {
		return err
	}

	reqUpdate := c.apiClient.LoadTestsAPI.LoadTestsScriptUpdate(ctx, testID).
		Body(io.NopCloser(bytes.NewReader(buf.Bytes()))).
		XStackId(c.stackID)

	httpRes, rerr := reqUpdate.Execute()
	defer closeResponse(httpRes, &err)
	if err := CheckResponse(httpRes, rerr); err != nil {
		return err
	}

	return nil
}

// GetCloudTestByName retrieves a cloud test by its name within the specified project.
func (c *Client) GetCloudTestByName(name string, projectID int64) (lt *k6cloud.LoadTestApiModel, err error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)

	projectID32, err := toInt32(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	req := c.apiClient.LoadTestsAPI.ProjectsLoadTestsRetrieve(ctx, projectID32).
		XStackId(c.stackID).
		Name(name)

	loadTests, httpRes, rerr := req.Execute()
	defer closeResponse(httpRes, &err)
	if err := CheckResponse(httpRes, rerr); err != nil {
		return nil, err
	}

	for _, test := range loadTests.Value {
		if test.Name == name {
			return &test, nil
		}
	}

	return nil, fmt.Errorf("failed to retrieve existing test with name %q", name)
}

// CreateOrUpdateCloudTest creates a new cloud test or updates an existing one
// if a test with the same name already exists.
func (c *Client) CreateOrUpdateCloudTest(
	name string, projectID int64, arc *lib.Archive,
) (*k6cloud.LoadTestApiModel, error) {
	test, err := c.CreateCloudTest(name, projectID, arc)
	if err != nil {
		var rErr ResponseError
		if !errors.As(err, &rErr) || rErr.Response.StatusCode != http.StatusConflict {
			return nil, err
		}

		// Test with the same name already exists, we need to update it
		test, err = c.GetCloudTestByName(name, projectID)
		if err != nil {
			return nil, err
		}

		if err := c.updateCloudTest(test.Id, arc); err != nil {
			return nil, err
		}
	}

	return test, nil
}

// randomStrHex returns a hex string which can be used
// for session token id or idempotency key.
func randomStrHex() string {
	// 16 hex characters
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// StartCloudTestRun starts a new cloud test run for a given test.
func (c *Client) StartCloudTestRun(loadTestID int32) (ltr *k6cloud.TestRunApiModel, err error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)

	reqStart := c.apiClient.LoadTestsAPI.LoadTestsStart(ctx, loadTestID).
		XStackId(c.stackID).
		K6IdempotencyKey(randomStrHex())
	loadTestRun, httpRes, rerr := reqStart.Execute()
	defer closeResponse(httpRes, &err)
	if err := CheckResponse(httpRes, rerr); err != nil {
		return nil, err
	}

	return loadTestRun, nil
}

// CreateAndStartCloudTestRun creates a new cloud test (or updates it if it already exists) and starts a new test run.
func (c *Client) CreateAndStartCloudTestRun(
	name string, projectID int64, arc *lib.Archive,
) (*k6cloud.TestRunApiModel, error) {
	loadTest, err := c.CreateOrUpdateCloudTest(name, projectID, arc)
	if err != nil {
		return nil, err
	}

	loadTestRun, err := c.StartCloudTestRun(loadTest.Id)
	if err != nil {
		return nil, err
	}

	return loadTestRun, nil
}

// StopCloudTestRun tells the cloud to stop the test with the provided testRunID.
func (c *Client) StopCloudTestRun(testRunID int64) (err error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)

	testRunID32, err := toInt32(testRunID)
	if err != nil {
		return fmt.Errorf("invalid test run ID: %w", err)
	}

	req := c.apiClient.TestRunsAPI.TestRunsAbort(ctx, testRunID32).XStackId(c.stackID)
	httpRes, rerr := req.Execute()
	defer closeResponse(httpRes, &err)
	if err := CheckResponse(httpRes, rerr); err != nil {
		return err
	}

	return nil
}

// ValidateOptions sends the provided options to the cloud for validation.
func (c *Client) ValidateOptions(projectID int64, options lib.Options) (err error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)

	raw, rerr := json.Marshal(options)
	if rerr != nil {
		return rerr
	}
	var generic map[string]interface{}
	if rerr := json.Unmarshal(raw, &generic); rerr != nil {
		return rerr
	}

	projectID32, rerr := toInt32(projectID)
	if rerr != nil {
		return fmt.Errorf("invalid project ID: %w", rerr)
	}

	validateOptions := &k6cloud.ValidateOptionsRequest{
		ProjectId: *k6cloud.NewNullableInt32(&projectID32),
		Options: k6cloud.Options{
			AdditionalProperties: generic,
		},
	}

	req := c.apiClient.LoadTestsAPI.
		ValidateOptions(ctx).
		ValidateOptionsRequest(validateOptions).
		XStackId(c.stackID)
	_, httpRes, rerr := req.Execute()
	defer closeResponse(httpRes, &err)
	if err := CheckResponse(httpRes, rerr); err != nil {
		return err
	}

	return nil
}

// ValidateToken calls the endpoint to validate the Client's token and returns the result.
func (c *Client) ValidateToken(stackURL string) (_ *k6cloud.AuthenticationResponse, err error) {
	if stackURL == "" {
		return nil, errors.New("stack URL is required to validate token")
	}

	if _, err := url.Parse(stackURL); err != nil {
		return nil, fmt.Errorf("invalid stack URL: %w", err)
	}

	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)
	req := c.apiClient.AuthorizationAPI.
		Auth(ctx).
		XStackUrl(stackURL)

	resp, httpRes, rerr := req.Execute()
	defer closeResponse(httpRes, &err)
	if err := CheckResponse(httpRes, rerr); err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	return resp, err
}
