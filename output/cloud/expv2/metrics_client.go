package expv2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang/snappy"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"

	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/output/cloud/expv2/pbcloud"
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// MetricsClient is a Protobuf over HTTP client for sending
// the collected metrics from the Cloud output
// to the remote service.
type MetricsClient struct {
	httpClient httpDoer
	logger     logrus.FieldLogger
	token      string
	userAgent  string

	pushBufferPool sync.Pool
	baseURL        string
}

// NewMetricsClient creates and initializes a new MetricsClient.
func NewMetricsClient(logger logrus.FieldLogger, host string, token string) (*MetricsClient, error) {
	if host == "" {
		return nil, errors.New("host is required")
	}
	if token == "" {
		return nil, errors.New("token is required")
	}
	return &MetricsClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		logger:     logger,
		baseURL:    host + "/v2/metrics/",
		token:      token,
		userAgent:  "k6cloud/v" + consts.Version,
		pushBufferPool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}, nil
}

// Push pushes the provided metrics the given test run.
func (mc *MetricsClient) Push(ctx context.Context, referenceID string, samples *pbcloud.MetricSet) error {
	if referenceID == "" {
		return errors.New("TestRunID of the test is required")
	}
	start := time.Now()

	b, err := newRequestBody(samples)
	if err != nil {
		return err
	}

	buf, _ := mc.pushBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer mc.pushBufferPool.Put(buf)

	_, err = buf.Write(b)
	if err != nil {
		return err
	}
	// TODO: it is always the same
	// we don't expect to share this client across different refID
	// with a bit of effort we can find a way to just allocate once
	url := mc.baseURL + referenceID
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", mc.userAgent)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("K6-Metrics-Protocol-Version", "2.0")
	req.Header.Set("Authorization", "Token "+mc.token)

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response got an unexpected status code: %s", resp.Status)
	}
	mc.logger.WithField("t", time.Since(start)).WithField("size", len(b)).
		Debug("Pushed the collected metrics to the Cloud service")
	return nil
}

const payloadSizeLimit = 100 * 1024 // 100 KiB

func newRequestBody(data *pbcloud.MetricSet) ([]byte, error) {
	b, err := proto.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encoding metrics as Protobuf write request failed: %w", err)
	}
	if len(b) > payloadSizeLimit {
		return nil, fmt.Errorf("the Protobuf message is too large to be handled from the Cloud processor; "+
			"size: %d, limit: 100 KB", len(b))
	}
	if snappy.MaxEncodedLen(len(b)) < 0 {
		return nil, fmt.Errorf("the Protobuf message is too large to be handled by Snappy encoder; "+
			"size: %d, limit: %d", len(b), 0xffffffff)
	}
	return snappy.Encode(nil, b), nil
}
