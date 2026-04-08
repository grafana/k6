package cloudapi

import (
	"context"
	"errors"
	"fmt"
	"net/url"

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

	return resp, checkRequest(res, rerr, "validating token")
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

	return checkRequest(res, rerr, "validating options")
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
