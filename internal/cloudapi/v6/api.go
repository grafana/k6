package cloudapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

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
	if err != nil {
		return nil, err
	}

	if err := CheckResponse(httpRes); err != nil {
		return nil, err
	}

	return loadTest, nil
}

// StartCloudTestRun creates and starts a new cloud test run with the provided name and script archive.
func (c *Client) StartCloudTestRun(name string, projectID int64, arc *lib.Archive) (*k6cloud.TestRunApiModel, error) {
	loadTest, err := c.CreateCloudTest(name, projectID, arc)
	if err != nil {
		return nil, err
	}

	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)
	reqStart := c.apiClient.LoadTestsAPI.LoadTestsStart(ctx, loadTest.Id).XStackId(int32(c.stackID))
	loadTestRun, httpRes, err := reqStart.Execute()
	if err != nil {
		return nil, err
	}

	if err := CheckResponse(httpRes); err != nil {
		return nil, err
	}

	return loadTestRun, nil
}

// StopCloudTestRun tells the cloud to stop the test with the provided testRunID.
func (c *Client) StopCloudTestRun(testRunID int64) error {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)

	req := c.apiClient.TestRunsAPI.TestRunsAbort(ctx, int32(testRunID)).XStackId(int32(c.stackID))
	httpRes, err := req.Execute()
	if err != nil {
		return err
	}

	if err := CheckResponse(httpRes); err != nil {
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
	if err != nil {
		return err
	}

	if err := CheckResponse(httpRes); err != nil {
		return err
	}

	return nil
}

// ValidateToken calls the endpoint to validate the Client's token and returns the result.
func (c *Client) ValidateToken(stackURL string) (*k6cloud.AuthenticationResponse, error) {
	ctx := context.WithValue(context.Background(), k6cloud.ContextAccessToken, c.token)

	req := c.apiClient.AuthorizationAPI.
		Auth(ctx).
		XStackUrl(stackURL)

	resp, httpRes, err := req.Execute()
	defer func() {
		if httpRes != nil {
			_, _ = io.Copy(io.Discard, httpRes.Body)
			if cerr := httpRes.Body.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}
	}()

	if err != nil {
		var apiErr *k6cloud.GenericOpenAPIError
		if !errors.As(err, &apiErr) {
			return nil, err
		}
	}

	if err := CheckResponse(httpRes); err != nil {
		return nil, err
	}

	return resp, nil
}
