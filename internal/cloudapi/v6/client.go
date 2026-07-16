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

	"go.k6.io/k6/v2/internal/cloudapi/httperr"
	"go.k6.io/k6/v2/internal/cloudapi/httputil"
)

// Client handles communication with the k6 Cloud API.
type Client struct {
	apiClient *k6cloud.APIClient
	token     string
	stackID   int32
	baseURL   string

	logger logrus.FieldLogger
}

// NewSDKConfiguration builds a *k6cloud.Configuration for a client backed by
// the vendored k6cloud-openapi SDK, shared by this package and
// internal/cloudapi/provisioning: the same UserAgent convention, retry
// policy, and body-reset-on-retry workaround for the SDK's own retry loop.
func NewSDKConfiguration(host, version, serverDescription string, timeout time.Duration) *k6cloud.Configuration {
	cfg := k6cloud.NewConfiguration()
	cfg.UserAgent = "k6cloud/" + version
	cfg.Servers = k6cloud.ServerConfigurations{
		{
			URL:         host,
			Description: serverDescription,
		},
	}
	cfg.HTTPClient = &http.Client{
		Timeout:   timeout,
		Transport: httputil.BodyResetTransport{Base: http.DefaultTransport},
	}
	cfg.MaxRetries = httputil.MaxRetries
	cfg.RetryInterval = httputil.RetryInterval
	return cfg
}

// NewClient return a new client for the cloud API
func NewClient(logger logrus.FieldLogger, token, host, version string, timeout time.Duration) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required to create cloud API client")
	}

	cfg := NewSDKConfiguration(host, version, "Global k6 Cloud API.", timeout)

	c := &Client{
		apiClient: k6cloud.NewAPIClient(cfg),
		token:     token,
		baseURL:   fmt.Sprintf("%s/cloud/v6", host),
		logger:    logger,
	}
	return c, nil
}

// SetStackID sets the stack ID for the client.
func (c *Client) SetStackID(stackID int32) {
	c.stackID = stackID
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
		if classified := httperr.ClassifyStatus(r.StatusCode); classified != nil {
			return classified
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
