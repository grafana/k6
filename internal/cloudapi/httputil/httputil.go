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

// CloseResponse drains res.Body to io.Discard and then closes it.
//
// Draining the body before closing it allows the underlying connection to
// be returned to the client's connection pool and reused for a subsequent
// request; closing an unread body forces the transport to discard the
// connection instead. See https://pkg.go.dev/net/http#Response for details.
//
// If closing the body fails and *rerr is nil, the close error is assigned
// to *rerr. An error already present in *rerr (e.g. one produced while
// handling the response) is never overwritten.
func CloseResponse(res *http.Response, rerr *error) {
	if res == nil || res.Body == nil {
		return
	}

	_, _ = io.Copy(io.Discard, res.Body)
	if err := res.Body.Close(); err != nil && *rerr == nil {
		*rerr = err
	}
}

// ToInt32 converts v to int32, returning an error if it overflows the
// int32 range instead of truncating it.
func ToInt32(v int64) (int32, error) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, fmt.Errorf("value %d overflows int32", v)
	}
	return int32(v), nil
}
