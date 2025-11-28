package cloudapi

import (
	"context"
	"errors"
	"io"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
)

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
