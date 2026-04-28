package expv2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/klauspost/compress/snappy"
	"google.golang.org/protobuf/proto"

	"go.k6.io/k6/v2/internal/output/cloud/expv2/pbcloud"
)

// httpDoer performs an HTTP request and optionally decodes the response body into v.
// It is implemented by cloudapi.Client (legacy path) and netHTTPClient (provisioning path).
type httpDoer interface {
	Do(req *http.Request, v any) error
}

// metricsClient is a Protobuf over HTTP client for sending
// the collected metrics from the Cloud output
// to the remote service.
type metricsClient struct {
	httpClient   httpDoer
	url          string
	testRunToken string
}

// newMetricsClient creates and initializes a new MetricsClient.
// pushURL must be fully resolved by the caller before calling this function.
func newMetricsClient(
	c httpDoer, pushURL string, testRunID string, testRunToken string,
) (*metricsClient, error) {
	if testRunID == "" {
		return nil, errors.New("TestRunID of the test is required")
	}
	if pushURL == "" {
		return nil, errors.New("metrics push URL is required")
	}

	return &metricsClient{
		httpClient:   c,
		url:          pushURL,
		testRunToken: testRunToken,
	}, nil
}

// Push the provided metrics for the given test run ID.
func (mc *metricsClient) push(samples *pbcloud.MetricSet) error {
	b, err := newRequestBody(samples)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, mc.url, io.NopCloser(bytes.NewReader(b)))
	if err != nil {
		return err
	}

	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(b)), nil
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("K6-Metrics-Protocol-Version", "2.0")

	if mc.testRunToken != "" {
		req.Header.Set("Authorization", "Bearer "+mc.testRunToken)
	}

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
