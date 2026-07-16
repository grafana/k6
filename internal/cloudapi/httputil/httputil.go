// Package httputil contains small HTTP helpers shared by the various
// cloud API clients (the legacy v1 client in cloudapi, the v6 client in
// internal/cloudapi/v6, and the provisioning client in
// internal/cloudapi/provisioning).
package httputil

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	// MaxRetries is the default number of retry attempts for cloud API requests.
	MaxRetries = 3
	// RetryInterval is the default cloud request retry interval.
	RetryInterval = 500 * time.Millisecond
)

// CloseResponse drains res.Body before closing it, so the connection can be
// reused instead of discarded, then assigns any close error to *rerr unless
// it's already set.
func CloseResponse(res *http.Response, rerr *error) {
	if res == nil || res.Body == nil {
		return
	}

	_, _ = io.Copy(io.Discard, res.Body)
	if err := res.Body.Close(); err != nil && *rerr == nil {
		*rerr = err
	}
}

// ToInt32 errors on overflow instead of truncating.
func ToInt32(v int64) (int32, error) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, fmt.Errorf("value %d overflows int32", v)
	}
	return int32(v), nil
}
