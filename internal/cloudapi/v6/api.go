package cloudapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"

	"go.k6.io/k6/v2/lib"
)

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
