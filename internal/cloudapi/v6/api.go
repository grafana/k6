package cloudapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"go.k6.io/k6/lib"
)

// CreateCloudTest creates a new cloud test with the provided name and script archive.
func (c *Client) CreateCloudTest(name string, projectID int64, arc *lib.Archive) (*k6cloud.LoadTestApiModel, error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)
	var buf bytes.Buffer
	if err := arc.Write(&buf); err != nil {
		return nil, err
	}

	reqCreate := c.apiClient.LoadTestsAPI.ProjectsLoadTestsCreate(ctx, int32(projectID)).
		Name(name).
		Script(io.NopCloser(bytes.NewReader(buf.Bytes()))).
		XStackId(int32(c.stackID))

	loadTest, httpRes, err := reqCreate.Execute()
	if err := CheckResponse(httpRes, err); err != nil {
		return nil, err
	}

	return loadTest, nil
}

// StartCloudTestRun creates and starts a new cloud test run with the provided name and script archive.
func (c *Client) StartCloudTestRun(loadTestId int32) (*k6cloud.TestRunApiModel, error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)
	reqStart := c.apiClient.LoadTestsAPI.LoadTestsStart(ctx, loadTestId).XStackId(int32(c.stackID))
	loadTestRun, httpRes, err := reqStart.Execute()
	if err := CheckResponse(httpRes, err); err != nil {
		return nil, err
	}

	return loadTestRun, nil
}

// GetCloudTestByName retrieves a cloud test by its name within the specified project.
func (c *Client) GetCloudTestByName(name string, projectID int64) (*k6cloud.LoadTestApiModel, error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)
	// TODO: Replace with ProjectLoadTestsList(ctx, int32(projectID)) when name filter is added to the endpoint
	req := c.apiClient.LoadTestsAPI.LoadTestsList(ctx).
		XStackId(int32(c.stackID)).
		Name(name)

	loadTests, httpRes, err := req.Execute()
	if err := CheckResponse(httpRes, err); err != nil {
		return nil, err
	}

	for _, lt := range loadTests.Value {
		if lt.Name == name {
			return &lt, nil
		}
	}

	return nil, nil
}

// CreateAndStartCloudTestRun creates a new cloud test (or retrieve it if it already exists) and starts a new test run.
func (c *Client) CreateAndStartCloudTestRun(name string, projectID int64, arc *lib.Archive) (*k6cloud.TestRunApiModel, error) {
	loadTest, err := c.CreateCloudTest(name, projectID, arc)
	if err != nil {
		var rErr ResponseError
		if !errors.As(err, &rErr) || rErr.Response.StatusCode != 409 {
			return nil, err
		}

		// Test with the same name already exists
		test, err := c.GetCloudTestByName(name, projectID)
		if err != nil {
			return nil, err
		}
		if test == nil {
			return nil, fmt.Errorf("failed to retrieve existing test with name %q", name)
		}
		loadTest = test
	}

	loadTestRun, err := c.StartCloudTestRun(loadTest.Id)
	if err != nil {
		return nil, err
	}

	return loadTestRun, nil
}

// StopCloudTestRun tells the cloud to stop the test with the provided testRunID.
func (c *Client) StopCloudTestRun(testRunID int64) error {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)

	req := c.apiClient.TestRunsAPI.TestRunsAbort(ctx, int32(testRunID)).XStackId(int32(c.stackID))
	httpRes, err := req.Execute()
	if err := CheckResponse(httpRes, err); err != nil {
		return err
	}

	return nil
}

// ValidateOptions sends the provided options to the cloud for validation.
func (c *Client) ValidateOptions(projectID int64, options lib.Options) error {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)

	raw, err := json.Marshal(options)
	if err != nil {
		return err
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}

	projectIDint32 := int32(projectID)
	validateOptions := &k6cloud.ValidateOptionsRequest{
		ProjectId: *k6cloud.NewNullableInt32(&projectIDint32),
		Options: k6cloud.Options{
			AdditionalProperties: generic,
		},
	}

	req := c.apiClient.LoadTestsAPI.ValidateOptions(ctx).ValidateOptionsRequest(validateOptions)
	_, httpRes, err := req.Execute()
	if err := CheckResponse(httpRes, err); err != nil {
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
	defer func() {
		if httpRes != nil {
			_, _ = io.Copy(io.Discard, httpRes.Body)
			if cerr := httpRes.Body.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}
	}()

	if rerr != nil {
		var apiErr *k6cloud.GenericOpenAPIError
		if !errors.As(rerr, &apiErr) {
			return nil, fmt.Errorf("failed to validate token: %w", rerr)
		}
	}

	if err := CheckResponse(httpRes); err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	return resp, err
}
