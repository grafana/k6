// Package httperr contains small, shared HTTP-status-code-related error
// helpers used by k6 Cloud's several API clients (e.g. the legacy v1 client
// in cloudapi and the v6 client in internal/cloudapi/v6).
//
// It intentionally stays tiny: it only captures the part of those clients'
// CheckResponse-style functions that is genuinely identical across them -
// classifying a 401/403 HTTP status code into a friendly sentinel error.
// Everything else about decoding a cloud API error response body differs
// between API versions and is left to each client to implement on its own.
package httperr

import (
	"errors"
	"net/http"
)

var (
	// ErrNotAuthenticated is returned when the k6 Cloud API responds with
	// HTTP 401 Unauthorized, meaning the configured token failed to
	// authenticate the request.
	ErrNotAuthenticated = errors.New("failed to authenticate with k6 Cloud")

	// ErrNotAuthorized is returned when the k6 Cloud API responds with
	// HTTP 403 Forbidden, meaning the request was authenticated but is not
	// allowed to perform the requested operation.
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
