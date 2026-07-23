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

// metricsHTTPClient is the HTTP transport contract used by metricsClient.
type metricsHTTPClient interface {
	Do(req *http.Request, v any) error
}

// metricsClient is a Protobuf over HTTP client for sending
// the collected metrics from the Cloud output
// to the remote service.
type metricsClient struct {
	httpClient metricsHTTPClient
	url        string
}

// deriveMetricsURL builds the v2 metrics-ingestion URL from the Cloud
// service base URL, used when no explicit push URL was provided.
//
// The cloudapi.Client works across different versions of the API: test
// lifecycle management is under /v1 while metrics ingestion is /v2.
// The client has /v1 hard-coded, so we trim it and replace with /v2.
// A versioned client would be better but it would require a breaking
// change, and other services (e.g. k6-operator) depend on it, so we want
// to stabilize the API first.
func deriveMetricsURL(baseURL, testRunID string) (string, error) {
	if !strings.HasSuffix(baseURL, "/v1") {
		return "", errors.New("a /v1 suffix is expected in the Cloud service's BaseURL path")
	}
	if testRunID == "" {
		return "", errors.New("TestRunID of the test is required")
	}
	return strings.TrimSuffix(baseURL, "/v1") + "/v2/metrics/" + testRunID, nil
}

// newMetricsClientWithURL builds a metricsClient with an explicit push URL.
// It is the single metricsClient constructor: callers that need the
// host-derived URL compute it via deriveMetricsURL first.
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
