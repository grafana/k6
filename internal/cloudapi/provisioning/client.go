package provisioning

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/sirupsen/logrus"

	v6 "go.k6.io/k6/v2/internal/cloudapi/v6"
)

const (
	// RetryInterval is the default cloud request retry interval.
	RetryInterval = 500 * time.Millisecond
	// MaxRetries specifies max retry attempts.
	MaxRetries = 3
)

// Client handles communication with the provisioning and v6 cloud APIs.
type Client struct {
	apiClient *k6cloud.APIClient
	v6Client  *v6.Client
	token     string
	stackID   int32
	host      string
	version   string

	logger logrus.FieldLogger
}

// NewClient returns a new provisioning Client. It internally constructs
// both a k6cloud.APIClient (for provisioning endpoints) and a v6.Client
// (for v6 operations like FetchTest). The stackID identifies the
// Grafana Cloud stack; a value of 0 means no stack is configured and
// stack-scoped requests will be issued without the X-Stack-Id header.
func NewClient(
	logger logrus.FieldLogger,
	token, host, version string,
	stackID int32,
	timeout time.Duration,
) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required to create provisioning API client")
	}

	cfg := k6cloud.NewConfiguration()
	cfg.UserAgent = "k6cloud/" + version
	cfg.Servers = k6cloud.ServerConfigurations{
		{
			URL:         host,
			Description: "k6 Cloud API (provisioning).",
		},
	}
	cfg.HTTPClient = &http.Client{
		Timeout:   timeout,
		Transport: http.DefaultTransport,
	}
	cfg.MaxRetries = MaxRetries
	cfg.RetryInterval = RetryInterval

	v6c, err := v6.NewClient(logger, token, host, version, timeout)
	if err != nil {
		return nil, fmt.Errorf("creating v6 client: %w", err)
	}

	c := &Client{
		apiClient: k6cloud.NewAPIClient(cfg),
		v6Client:  v6c,
		token:     token,
		stackID:   stackID,
		host:      host,
		version:   version,
		logger:    logger,
	}
	if stackID != 0 {
		// Configure the X-Stack-Id default header so that all
		// provisioning API requests include it.
		c.apiClient.GetConfig().DefaultHeader["X-Stack-Id"] = strconv.FormatInt(int64(stackID), 10)
		// Propagate the stack ID to the embedded v6 client so
		// CreateOrFindLoadTest sends the correct X-Stack-Id header.
		// stackID is already int32, so the widening never overflows.
		if err := c.v6Client.SetStackID(int64(stackID)); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// authCtx returns a context carrying the API access token.
func (c *Client) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, k6cloud.ContextAccessToken, c.token)
}

// doWithRetry executes the given HTTP request, retrying on 5xx status
// codes and transport errors up to MaxRetries times. The request body
// is reset from req.GetBody before each attempt so a retried request
// with a body is not sent empty. 4xx errors are NOT retried.
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	httpClient := c.apiClient.GetConfig().HTTPClient

	var (
		lastErr  error
		lastResp *http.Response
	)

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		// The vendored SDK resets the body on its own internal retries,
		// but this direct Do loop must do it itself.
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}

		lastResp, lastErr = httpClient.Do(req) //nolint:gosec
		if lastErr != nil {
			if attempt < MaxRetries {
				time.Sleep(RetryInterval)
				continue
			}
			break
		}

		if lastResp.StatusCode >= 500 && attempt < MaxRetries {
			_, _ = io.Copy(io.Discard, lastResp.Body)
			_ = lastResp.Body.Close()
			time.Sleep(RetryInterval)
			continue
		}

		break
	}

	return lastResp, lastErr
}
