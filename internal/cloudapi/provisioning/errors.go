package provisioning

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"go.k6.io/k6/v2/internal/cloudapi/httperr"
)

var errUnknown = errors.New("an error occurred communicating with the provisioning API")

// ResponseError represents an error returned by the provisioning API.
type ResponseError struct {
	StatusCode int
	Body       string
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("provisioning API error (%d): %s", e.StatusCode, e.Body)
}

// CheckResponse checks an HTTP response from the provisioning API.
// It returns nil if the status code is in the 2xx range, otherwise it
// returns a *ResponseError with the status code and response body.
func CheckResponse(resp *http.Response) error {
	if resp == nil {
		return errUnknown
	}

	if c := resp.StatusCode; c >= 200 && c <= 299 {
		return nil
	}

	// Classify 401/403 to the shared sentinels so provisioning reports
	// auth failures consistently with the v1 and v6 clients.
	if classified := httperr.ClassifyStatus(resp.StatusCode); classified != nil {
		return classified
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return errUnknown
	}

	return &ResponseError{
		StatusCode: resp.StatusCode,
		Body:       string(data),
	}
}
