package cloudapi

import (
	"encoding/json"
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
	stackID   int64
	baseURL   string

	logger logrus.FieldLogger

	retries       int
	retryInterval time.Duration
}

// NewClient return a new client for the cloud API
func NewClient(logger logrus.FieldLogger, token, host, version string, timeout time.Duration) *Client {
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
	return c
}

// SetStackID sets the stack ID for the client.
func (c *Client) SetStackID(stackID int64) {
	c.stackID = stackID
}

// BaseURL returns configured host.
func (c *Client) BaseURL() string {
	return c.baseURL
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
