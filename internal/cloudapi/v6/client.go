package cloudapi

import (
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
	stackID   int32
	baseURL   string

	logger logrus.FieldLogger
}

// NewClient return a new client for the cloud API
func NewClient(logger logrus.FieldLogger, token, host, version string, timeout time.Duration) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required to create cloud API client")
	}

	cfg := k6cloud.NewConfiguration()
	cfg.UserAgent = "k6cloud/" + version
	cfg.Servers = k6cloud.ServerConfigurations{
		{
			URL:         host,
			Description: "Global k6 Cloud API.",
		},
	}
	cfg.HTTPClient = &http.Client{
		Timeout:   timeout,
		Transport: http.DefaultTransport,
	}
	cfg.MaxRetries = MaxRetries
	cfg.RetryInterval = RetryInterval

	c := &Client{
		apiClient: k6cloud.NewAPIClient(cfg),
		token:     token,
		baseURL:   fmt.Sprintf("%s/cloud/v6", host),
		logger:    logger,
	}
	return c, nil
}

// SetStackID sets the stack ID for the client. It returns an error if
// stackID does not fit in the int32 range the underlying SDK requires for
// the X-Stack-Id header.
func (c *Client) SetStackID(stackID int64) error {
	if stackID < math.MinInt32 || stackID > math.MaxInt32 {
		return fmt.Errorf("stack ID %d overflows int32", stackID)
	}
	c.stackID = int32(stackID)
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
		var aerr *k6cloud.GenericOpenAPIError
		if !errors.As(err, &aerr) {
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
