// Package httperr contains HTTP-status-code error helpers shared by k6
// Cloud's API clients (cloudapi, internal/cloudapi/v6). It only covers the
// part of their CheckResponse-style functions that's genuinely identical
// across them - classifying 401/403 - everything else about decoding an
// error response body differs between API versions.
package httperr

import (
	"errors"
	"net/http"
)

var (
	// ErrNotAuthenticated maps HTTP 401.
	ErrNotAuthenticated = errors.New("failed to authenticate with k6 Cloud")
	// ErrNotAuthorized maps HTTP 403.
	ErrNotAuthorized = errors.New("not allowed to upload result to k6 Cloud")
)

// ClassifyStatus returns ErrNotAuthenticated for HTTP 401, ErrNotAuthorized
// for HTTP 403, and nil for any other status code.
func ClassifyStatus(statusCode int) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return ErrNotAuthenticated
	case http.StatusForbidden:
		return ErrNotAuthorized
	default:
		return nil
	}
}
