package cloudapi

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
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
