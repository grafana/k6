package provisioning

import (
	"io"
	"net/http"
	"time"

	"go.k6.io/k6/v2/internal/cloudapi/httputil"
)

// doHTTPWithRetry retries on transport errors, 5xx, and 429 - matching the
// vendored SDK's own retry predicate. resetBody, if non-nil, rewinds the
// request body before each retry; pass nil when the client's Transport
// already replays it (e.g. bodyResetTransport).
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
