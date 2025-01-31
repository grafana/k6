// Package remote implements the Prometheus remote write protocol.
package remote

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"go.k6.io/k6/internal/output/prometheusrw/sigv4"

	prompb "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	"github.com/klauspost/compress/snappy"
	"google.golang.org/protobuf/proto"
)

// HTTPConfig holds the config for the HTTP client.
type HTTPConfig struct {
	Timeout   time.Duration
	TLSConfig *tls.Config
	BasicAuth *BasicAuth
	SigV4     *sigv4.Config
	Headers   http.Header
}

// BasicAuth holds the config for basic authentication.
type BasicAuth struct {
	Username, Password string
}

// WriteClient is a client implementation of the Prometheus remote write protocol.
// It follows the specs defined by the official design document:
// https://docs.google.com/document/d/1LPhVRSFkGNSuU1fBd81ulhsCPR4hkSZyyBj1SZ8fWOM
type WriteClient struct {
	hc  *http.Client
	url *url.URL
	cfg *HTTPConfig
}

// NewWriteClient creates a new WriteClient.
func NewWriteClient(endpoint string, cfg *HTTPConfig) (*WriteClient, error) {
	if cfg == nil {
		cfg = &HTTPConfig{}
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	wc := &WriteClient{
		hc: &http.Client{
			Timeout: cfg.Timeout,
		},
		url: u,
		cfg: cfg,
	}
	if cfg.TLSConfig != nil {
		wc.hc.Transport = &http.Transport{
			TLSClientConfig: cfg.TLSConfig,
		}
	}
	if cfg.SigV4 != nil {
		tripper, err := sigv4.NewRoundTripper(cfg.SigV4, wc.hc.Transport)
		if err != nil {
			return nil, err
		}
		wc.hc.Transport = tripper
	}
	return wc, nil
}

// Store sends a batch of samples to the HTTP endpoint,
// the request is the proto marshaled and encoded.
func (c *WriteClient) Store(ctx context.Context, series []*prompb.TimeSeries) error {
	b, err := newWriteRequestBody(series)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.url.String(), bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create new HTTP request failed: %w", err)
	}
	if c.cfg.BasicAuth != nil {
		req.SetBasicAuth(c.cfg.BasicAuth.Username, c.cfg.BasicAuth.Password)
	}

	if len(c.cfg.Headers) > 0 {
		req.Header = c.cfg.Headers.Clone()
	}

	req.Header.Set("User-Agent", "k6-prometheus-rw-output")

	// They are mostly defined by the specs
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST request failed: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			panic(err)
		}
	}()

	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return err
	}

	return validateResponseStatus(resp.StatusCode)
}

func newWriteRequestBody(series []*prompb.TimeSeries) ([]byte, error) {
	b, err := proto.Marshal(&prompb.WriteRequest{
		Timeseries: series,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding series as protobuf write request failed: %w", err)
	}
	if snappy.MaxEncodedLen(len(b)) < 0 {
		return nil, fmt.Errorf("the protobuf message is too large to be handled by Snappy encoder; "+
			"size: %d, limit: %d", len(b), math.MaxUint32)
	}
	return snappy.Encode(nil, b), nil
}

func validateResponseStatus(code int) error {
	if code >= http.StatusOK && code < 300 {
		return nil
	}

	return fmt.Errorf("got status code: %d instead expected a 2xx successful status code", code)
}
