package cloudapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	stackID   int32
	baseURL   string

	logger logrus.FieldLogger

	retries       int
	retryInterval time.Duration
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
	}

	c := &Client{
		apiClient:     k6cloud.NewAPIClient(cfg),
		token:         token,
		baseURL:       fmt.Sprintf("%s/cloud/v6", host),
		retries:       MaxRetries,
		retryInterval: RetryInterval,
		logger:        logger,
	}
	return c, nil
}

// SetStackID sets the stack ID for the client.
func (c *Client) SetStackID(stackID int64) error {
	stackID32, err := toInt32(stackID)
	if err != nil {
		return fmt.Errorf("invalid stack ID: %w", err)
	}
	c.stackID = stackID32
	return nil
}

// BaseURL returns configured host.
func (c *Client) BaseURL() string {
	return c.baseURL
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

// closeResponse is a helper that ensures the response body is properly closed.
// It should be called with defer immediately after receiving a response.
func closeResponse(r *http.Response, errPtr *error) {
	if r == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	if cerr := r.Body.Close(); cerr != nil && *errPtr == nil {
		*errPtr = cerr
	}
}

// toInt32 safely converts an int64 to int32, returning an error if overflow would occur.
func toInt32(val int64) (int32, error) {
	if val < -2147483648 || val > 2147483647 {
		return 0, fmt.Errorf("value %d overflows int32", val)
	}
	return int32(val), nil
}
