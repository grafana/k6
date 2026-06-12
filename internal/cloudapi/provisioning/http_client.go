package provisioning

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPClient is a Bearer-authenticated HTTP layer used by the cloud
// Output's expv2 metrics push when in provisioning mode. Signs
// requests with the scoped test_run_token, retries on 5xx and
// transport errors, decodes JSON responses into a caller-provided
// struct. It implements only Do — not BaseURL — because the metrics
// URL is set explicitly by the caller (no client-side derivation).
type HTTPClient struct {
	httpClient *http.Client
	token      string // scoped test_run_token
	version    string // for User-Agent
	logger     logrus.FieldLogger
}

// NewHTTPClient constructs an HTTPClient for metrics push and notify
// in provisioning mode. version is used for the User-Agent header,
// matching the v1/v6 client convention `k6cloud/<version>`.
func NewHTTPClient(httpClient *http.Client, token, version string, logger logrus.FieldLogger) *HTTPClient {
	return &HTTPClient{httpClient: httpClient, token: token, version: version, logger: logger}
}

// Do executes the request with Bearer auth, retries on 5xx and
// transport errors, and decodes the response body into v if non-nil.
func (p *HTTPClient) Do(req *http.Request, v any) error {
	// Ensure GetBody is set so the body can be replayed on retries.
	if req.Body != nil && req.GetBody == nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("reading request body: %w", err)
		}
		_ = req.Body.Close()

		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
		req.Body, _ = req.GetBody()
		req.ContentLength = int64(len(body))
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("User-Agent", "k6cloud/"+p.version)

	var (
		lastErr  error
		lastResp *http.Response
	)

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		lastResp, lastErr = p.httpClient.Do(req) //nolint:gosec
		if lastErr != nil {
			if attempt < MaxRetries {
				time.Sleep(RetryInterval)
				if req.GetBody != nil {
					req.Body, _ = req.GetBody()
				}
				continue
			}
			break
		}

		if lastResp.StatusCode >= 500 && attempt < MaxRetries {
			_, _ = io.Copy(io.Discard, lastResp.Body)
			_ = lastResp.Body.Close()
			time.Sleep(RetryInterval)
			if req.GetBody != nil {
				req.Body, _ = req.GetBody()
			}
			continue
		}

		break
	}

	if lastErr != nil {
		return lastErr
	}

	defer func() {
		_, _ = io.Copy(io.Discard, lastResp.Body)
		_ = lastResp.Body.Close()
	}()

	if err := CheckResponse(lastResp); err != nil {
		return err
	}

	if v != nil {
		if err := json.NewDecoder(lastResp.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}
