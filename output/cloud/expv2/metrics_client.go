package expv2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/snappy"
	"google.golang.org/protobuf/proto"

	"go.k6.io/k6/v2/internal/output/cloud/expv2/pbcloud"
)

// metricsHTTPClient is the minimal HTTP transport contract used by
// metricsClient post-construction. The legacy URL-deriving constructor
// requires the extended metricsHTTPClientWithBaseURL; the explicit-URL
// constructor (added later) takes only this smaller interface.
type metricsHTTPClient interface {
	Do(req *http.Request, v any) error
}

// metricsHTTPClientWithBaseURL extends metricsHTTPClient with BaseURL,
// used by newMetricsClient to derive the metrics push URL from the host.
type metricsHTTPClientWithBaseURL interface {
	metricsHTTPClient
	BaseURL() string
}

// metricsClient is a Protobuf over HTTP client for sending
// the collected metrics from the Cloud output
// to the remote service.
type metricsClient struct {
	httpClient metricsHTTPClient // was: metricsHTTPClientWithBaseURL
	url        string
}

// newMetricsClient creates and initializes a new MetricsClient.
func newMetricsClient(c metricsHTTPClientWithBaseURL, testRunID string) (*metricsClient, error) {
	// The cloudapi.Client works across different versions of the API, the test
	// lifecycle management is under /v1 instead the metrics ingestion is /v2.
	// Unfortunately, the current client has v1 hard-coded so we need to trim the wrong path
	// to be able to replace it with the correct one.
	// A versioned client would be better but it would require a breaking change
	// and considering that other services (e.g. k6-operator) depend on it,
	// we want to stabilize the API before.
	u := c.BaseURL()
	if !strings.HasSuffix(u, "/v1") {
		return nil, errors.New("a /v1 suffix is expected in the Cloud service's BaseURL path")
	}
	if testRunID == "" {
		return nil, errors.New("TestRunID of the test is required")
	}
	return &metricsClient{
		httpClient: c,
		url:        strings.TrimSuffix(u, "/v1") + "/v2/metrics/" + testRunID,
	}, nil
}

// newMetricsClientWithURL builds a metricsClient with an explicit push
// URL and a smaller metricsHTTPClient (no BaseURL derivation). Used by
// the provisioning-mode metrics push, where the URL is returned by the
// API and not derived from a host.
func newMetricsClientWithURL(c metricsHTTPClient, url string) (*metricsClient, error) {
	if url == "" {
		return nil, errors.New("metrics push URL is required")
	}
	return &metricsClient{
		httpClient: c,
		url:        url,
	}, nil
}

// Push the provided metrics for the given test run ID. The context cancels the
// underlying HTTP request so a stuck push cannot block k6 shutdown.
func (mc *metricsClient) push(ctx context.Context, samples *pbcloud.MetricSet) error {
	b, err := newRequestBody(samples)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, mc.url, io.NopCloser(bytes.NewReader(b)))
	if err != nil {
		return err
	}

	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(b)), nil
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("K6-Metrics-Protocol-Version", "2.0")

	err = mc.httpClient.Do(req, nil)
	if err != nil {
		return err
	}

	return nil
}

func newRequestBody(data *pbcloud.MetricSet) ([]byte, error) {
	b, err := proto.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encoding metrics as Protobuf write request failed: %w", err)
	}
	// TODO: use the framing format
	// https://github.com/google/snappy/blob/main/framing_format.txt
	// It can be done replacing the encode with
	// https://pkg.go.dev/github.com/klauspost/compress/snappy#NewBufferedWriter
	if snappy.MaxEncodedLen(len(b)) < 0 {
		return nil, fmt.Errorf("the Protobuf message is too large to be handled by Snappy encoder; "+
			"size: %d, limit: %d", len(b), 0xffffffff)
	}
	return snappy.Encode(nil, b), nil
}
