package provisioning

import (
	"io"
	"net/http"
	"time"

	"go.k6.io/k6/v2/internal/cloudapi/httputil"
)

// doHTTPWithRetry executes req via client, retrying on transport errors,
// 5xx responses, and 429 responses - the same predicate the vendored SDK
// uses for its own generated operations - up to httputil.MaxRetries
// attempts.
//
// If resetBody is non-nil, it's called before every retry attempt (not the
// first) to rewind a request body that isn't rewound automatically at the
// transport layer. Pass nil when client's Transport already replays the
// body itself (e.g. bodyResetTransport, as used by the SDK-backed
// *Client.apiClient).
func doHTTPWithRetry(client *http.Client, req *http.Request, resetBody func() error) (*http.Response, error) {
	var (
		lastErr  error
		lastResp *http.Response
	)

	for attempt := 1; attempt <= httputil.MaxRetries; attempt++ {
		if attempt > 1 && resetBody != nil {
			if err := resetBody(); err != nil {
				return nil, err
			}
		}

		lastResp, lastErr = client.Do(req) //nolint:gosec // caller closes lastResp on return
		retryableStatus := lastResp != nil &&
			(lastResp.StatusCode >= http.StatusInternalServerError || lastResp.StatusCode == http.StatusTooManyRequests)
		if attempt >= httputil.MaxRetries || (lastErr == nil && !retryableStatus) {
			break
		}

		if lastResp != nil {
			_, _ = io.Copy(io.Discard, lastResp.Body)
			_ = lastResp.Body.Close()
		}
		time.Sleep(httputil.RetryInterval)
	}

	return lastResp, lastErr
}
