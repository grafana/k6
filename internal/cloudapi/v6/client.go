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

const (
	// RetryInterval is the default cloud request retry interval
	RetryInterval = 500 * time.Millisecond
	// MaxRetries specifies max retry attempts
	MaxRetries = 3
)

// Client handles communication with the k6 Cloud API.
type Client struct {
	apiClient *k6cloud.APIClient
	token     string
	stackID   int64
	baseURL   string

	logger logrus.FieldLogger
}

// NewClient return a new client for the cloud API
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
		MaxRetries:       MaxRetries,
		RetryInterval:    RetryInterval,
	}

	c := &Client{
		apiClient: k6cloud.NewAPIClient(cfg),
		token:     token,
		baseURL:   fmt.Sprintf("%s/cloud/v6", host),
		logger:    logger,
	}
	return c, nil
}

// SetStackID sets the stack ID for the client.
func (c *Client) SetStackID(stackID int64) {
	c.stackID = stackID
}

// BaseURL returns configured host.
func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, k6cloud.ContextAccessToken, c.token)
}

func closeResponse(res *http.Response, err *error) {
	if res == nil {
		return
	}

	_, _ = io.Copy(io.Discard, res.Body)
	if cerr := res.Body.Close(); cerr != nil && err != nil && *err == nil {
		*err = cerr
	}
}

func checkRequest(res *http.Response, rerr error, action string) error {
	if rerr != nil {
		var gerr *k6cloud.GenericOpenAPIError
		if !errors.As(rerr, &gerr) {
			return fmt.Errorf("%s: %w", action, rerr)
		}
	}

	if err := CheckResponse(res); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	return nil
}

func checkInt32(name string, value int64) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf(
			"invalid %s: cannot be less than %d or greater than %d",
			name, math.MinInt32, math.MaxInt32,
		)
	}

	return int32(value), nil
}

// CheckResponse checks the parsed response.
// It returns nil if the code is in the successful range,
// otherwise it tries to parse the body and return a parsed error.
func CheckResponse(r *http.Response) error {
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
	err = json.Unmarshal(data, &payload)
	if err == nil {
		payload.Response = r
		return payload
	}
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
