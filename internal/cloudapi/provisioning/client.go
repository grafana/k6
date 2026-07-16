package provisioning

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/sirupsen/logrus"

	v6 "go.k6.io/k6/v2/internal/cloudapi/v6"
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

	cfg := v6.NewSDKConfiguration(
		host, version, "k6 Cloud API (provisioning).", timeout,
		&bodyResetTransport{base: http.DefaultTransport},
	)

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
		c.v6Client.SetStackID(stackID)
	}
	return c, nil
}

// authCtx returns a context carrying the API access token.
func (c *Client) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, k6cloud.ContextAccessToken, c.token)
}

// bodyResetTransport resets req.Body from GetBody before each round trip.
// The vendored SDK retries 5xx/429 by re-calling Do on the same request
// without resetting its Body. After Connection: close the drained body
// causes "ContentLength=N with Body length 0".
type bodyResetTransport struct{ base http.RoundTripper }

func (rt *bodyResetTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.GetBody == nil {
		return rt.base.RoundTrip(req)
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	req.Body = body
	return rt.base.RoundTrip(req)
}

// doWithRetry executes the given HTTP request, retrying on 5xx/429 status
// codes and transport errors up to httputil.MaxRetries times (matching the
// vendored SDK's own retry predicate). On each retry the body is replayed
// via req.GetBody (reset at the transport layer by bodyResetTransport,
// which the client's Configuration is built with). 4xx errors other than
// 429 are NOT retried.
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	return doHTTPWithRetry(c.apiClient.GetConfig().HTTPClient, req, nil)
}
