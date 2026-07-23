package provisioning

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

// HTTPClient is the scoped-token Bearer HTTP layer injected into the
// cloud Output's expv2 metrics push (and the notify call) when running
// in provisioning mode. It signs requests with the scoped
// test_run_token, retries on 5xx and transport errors via the shared
// doWithRetry helper, and decodes JSON responses into a caller-provided
// struct. Unlike Client, it drives no provisioning/v6 orchestration and
// implements only Do — not BaseURL — because the metrics URL is set
// explicitly by the caller (no client-side derivation).
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

	resp, err := doWithRetry(p.httpClient, req)
	if err != nil {
		return err
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if err := CheckResponse(resp); err != nil {
		return err
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}
