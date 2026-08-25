package cloudapi

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"go.k6.io/k6/v2/internal/cloudapi/httperr"
)

var errUnknown = errors.New("an error occurred communicating with k6 Cloud")

// ResponseError represents an error cause by talking to the API
type ResponseError struct {
	Response *http.Response `json:"-"`

	Code        int                 `json:"code"`
	Message     string              `json:"message"`
	Details     map[string][]string `json:"details"`
	FieldErrors map[string][]string `json:"field_errors"`
	Errors      []string            `json:"errors"`
}

func contains(s []string, e string) bool {
	return slices.Contains(s, e)
}

func (e ResponseError) Error() string {
	msg := e.Message

	for _, v := range e.Errors {
		// atm: `errors` and `message` could be duplicated
		// TODO: remove condition when the API changes
		if v != msg {
			msg += "\n " + v
		}
	}

	// `e.Details` is the old API version
	// TODO: do not handle `details` when the old API becomes obsolete
	var details []string
	var detail string
	for k, v := range e.Details {
		detail = k + ": " + strings.Join(v, ", ")
		details = append(details, detail)
	}

	for k, v := range e.FieldErrors {
		detail = k + ": " + strings.Join(v, ", ")
		// atm: `details` and `field_errors` could be duplicated
		if !contains(details, detail) {
			details = append(details, detail)
		}
	}

	if len(details) > 0 {
		msg += "\n " + strings.Join(details, "\n")
	}

	var code string
	switch {
	case e.Code > 0 && e.Response != nil:
		code = fmt.Sprintf("%d/E%d", e.Response.StatusCode, e.Code)
	case e.Response != nil:
		code = fmt.Sprintf("%d", e.Response.StatusCode)
	case e.Code > 0:
		code = fmt.Sprintf("E%d", e.Code)
	}

	if len(code) > 0 {
		msg = fmt.Sprintf("(%s) %s", code, msg)
	}

	return msg
}

// Is returns true if target is httperr.ErrNotAuthenticated and the response status code is 401,
// or if target is httperr.ErrNotAuthorized and the response status code is 403.
func (e ResponseError) Is(target error) bool {
	if target == httperr.ErrNotAuthenticated && e.Response != nil && e.Response.StatusCode == http.StatusUnauthorized {
		return true
	}
	if target == httperr.ErrNotAuthorized && e.Response != nil && e.Response.StatusCode == http.StatusForbidden {
		return true
	}
	return false
}
