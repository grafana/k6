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

	"go.k6.io/k6/v2/internal/cloudapi/clientcfg"
	v6 "go.k6.io/k6/v2/internal/cloudapi/v6"
)

// Client orchestrates the provisioning and v6 cloud APIs for a
// local-execution test run: creating/finding the load test, starting the
// run, uploading the archive, polling for readiness, and posting the
// completion notification. It wraps a k6cloud.APIClient for the
// provisioning endpoints and an embedded v6.Client for test-run status
// queries. It is distinct from HTTPClient, which is the scoped-token
// Bearer HTTP layer injected into the expv2 metrics push.
type Client struct {
	apiClient *k6cloud.APIClient
	v6Client  *v6.Client
	token     string
	stackID   int64
	host      string
	version   string

	logger logrus.FieldLogger
}

// NewClient returns a new provisioning Client. It internally constructs
// both a k6cloud.APIClient (for provisioning endpoints) and a v6.Client
// (for v6 operations like FetchTest). The stackID identifies the
// Grafana Cloud stack and is required for the stack-scoped requests.
func NewClient(
	logger logrus.FieldLogger,
	token, host, version string,
	stackID int64,
	timeout time.Duration,
) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required to create provisioning API client")
	}

	cfg := clientcfg.New(host, version, "k6 Cloud API (provisioning).", timeout)

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
	// Propagate the stack ID to the embedded v6 client. v6.SetStackID
	// validates that it fits the int32 the SDK's X-Stack-Id requires —
	// that is the single place the int32 boundary is enforced.
	if err := c.v6Client.SetStackID(stackID); err != nil {
		return nil, err
	}
	// The provisioning endpoints carry the stack ID as a decimal-string
	// default header, so no int32 narrowing is needed here.
	c.apiClient.GetConfig().DefaultHeader["X-Stack-Id"] = strconv.FormatInt(stackID, 10)
	return c, nil
}

// authCtx returns a context carrying the API access token.
func (c *Client) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, k6cloud.ContextAccessToken, c.token)
}

// doWithRetry executes the request via the provisioning Client's HTTP
// client. It is a thin wrapper around the package-level doWithRetry; see
// that function for the retry semantics.
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	return doWithRetry(c.apiClient.GetConfig().HTTPClient, req)
}

// doWithRetry runs req through httpClient, retrying on 5xx responses and
// transport errors up to clientcfg.MaxRetries times, resetting the body
// from req.GetBody before each attempt so a retried request with a body is
// not sent empty. The retry wait honours req.Context() so a cancelled
// request stops waiting promptly. 4xx responses are NOT retried.
func doWithRetry(httpClient *http.Client, req *http.Request) (*http.Response, error) {
	var (
		lastErr  error
		lastResp *http.Response
	)

	for attempt := 1; attempt <= clientcfg.MaxRetries; attempt++ {
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
			if attempt < clientcfg.MaxRetries {
				select {
				case <-time.After(clientcfg.RetryInterval):
				case <-req.Context().Done():
					return lastResp, req.Context().Err()
				}
				continue
			}
			break
		}

		if lastResp.StatusCode >= 500 && attempt < clientcfg.MaxRetries {
			_, _ = io.Copy(io.Discard, lastResp.Body)
			_ = lastResp.Body.Close()
			select {
			case <-time.After(clientcfg.RetryInterval):
			case <-req.Context().Done():
				return lastResp, req.Context().Err()
			}
			continue
		}

		break
	}

	return lastResp, lastErr
}
