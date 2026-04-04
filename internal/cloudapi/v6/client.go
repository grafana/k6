package cloudapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/sirupsen/logrus"
)

// Client handles communication with the k6 Cloud API v6.
type Client struct {
	apiClient *k6cloud.APIClient
	token     string
	stackID   int32
	projectID int32
	baseURL   string

	logger logrus.FieldLogger
}

// NewClient return a new client for the cloud API.
func NewClient(logger logrus.FieldLogger, token, host, version string, timeout time.Duration) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required to create cloud API client")
	}

	cfg := &k6cloud.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     "k6cloud/" + version,
		Servers: k6cloud.ServerConfigurations{
			{
				URL:         host,
				Description: "Global k6 Cloud API.",
			},
		},
		OperationServers: map[string]k6cloud.ServerConfigurations{},
		HTTPClient:       &http.Client{Timeout: timeout},
	}

	return &Client{
		apiClient: k6cloud.NewAPIClient(cfg),
		token:     token,
		baseURL:   fmt.Sprintf("%s/cloud/v6", host),
		logger:    logger,
	}, nil
}

// SetStackID sets the stack ID for the client.
// Mandatory stack/project enforcement is handled by the separate
// PR #5737 (k6#5651), not by this migration. Zero is allowed here
// so existing flows that rely on server-side defaults continue to work.
func (c *Client) SetStackID(stackID int64) error {
	id, err := toInt32(stackID)
	if err != nil {
		return fmt.Errorf("invalid stack ID: %w", err)
	}
	c.stackID = id
	return nil
}

// SetProjectID sets the project ID for the client.
// Zero means "let the backend pick" — see k6-cloud#4281 which defines
// project resolution as: explicit > stack default > 0.
func (c *Client) SetProjectID(projectID int64) error {
	id, err := toInt32(projectID)
	if err != nil {
		return fmt.Errorf("invalid project ID: %w", err)
	}
	c.projectID = id
	return nil
}

// BaseURL returns configured host.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// authCtx returns a context enriched with the client's access token.
func (c *Client) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, k6cloud.ContextAccessToken, c.token)
}

// closeResponse ensures the response body is drained and closed.
func closeResponse(r *http.Response, errPtr *error) {
	if r == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	if cerr := r.Body.Close(); cerr != nil && *errPtr == nil {
		*errPtr = cerr
	}
}

// CheckResponse checks the parsed response.
// It returns nil if the code is in the successful range,
// otherwise it tries to parse the body and return a parsed error.
func CheckResponse(r *http.Response, err error) error {
	if err != nil {
		var cloudErr *k6cloud.GenericOpenAPIError
		if !errors.As(err, &cloudErr) {
			return err
		}
	}

	if r == nil {
		return errUnknown
	}

	if c := r.StatusCode; c >= 200 && c <= 299 {
		return nil
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	var payload ResponseError
	if err := json.Unmarshal(data, &payload); err != nil {
		if r.StatusCode == http.StatusUnauthorized {
			return errNotAuthenticated
		}
		if r.StatusCode == http.StatusForbidden {
			return errNotAuthorized
		}
		return fmt.Errorf(
			"unexpected HTTP error from %s: %d %s",
			r.Request.URL,
			r.StatusCode,
			http.StatusText(r.StatusCode),
		)
	}
	payload.Response = r
	return payload
}

// toInt32 safely converts an int64 to int32, returning an error if overflow would occur.
func toInt32(val int64) (int32, error) {
	if val < math.MinInt32 || val > math.MaxInt32 {
		return 0, fmt.Errorf("value %d overflows int32", val)
	}
	return int32(val), nil
}
