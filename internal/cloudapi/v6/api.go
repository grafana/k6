package cloudapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
)

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
