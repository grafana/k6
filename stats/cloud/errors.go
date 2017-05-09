package cloud

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

// ErrorResponse represents an error cause by talking to the API
type ErrorResponse struct {
	Response *http.Response
	Message  string
	Code     int
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("%d %v", e.Code, e.Message)
}

var (
	ErrNotAuthorized    = errors.New("Not allowed to upload result to Load Impact cloud")
	ErrNotAuthenticated = errors.New("Failed to authenticate with Load Impact cloud")
	ErrNotUnknown       = errors.New("An error occurred talking to Load Impact cloud")
)
