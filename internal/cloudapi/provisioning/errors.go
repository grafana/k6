package provisioning

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"

	"go.k6.io/k6/v2/internal/cloudapi/httperr"
)

var errUnknown = errors.New("an error occurred communicating with the provisioning API")

// ResponseError represents a parsed error response from the provisioning
// API. Its wire shape ({"error": {message, code, target, details}}) is the
// same ErrorResponseApiModel every vendored-SDK-backed client decodes on
// failure (see internal/cloudapi/v6's identically-shaped ResponseError).
type ResponseError struct {
	StatusCode int                   `json:"-"`
	APIError   k6cloud.ErrorApiModel `json:"error"`
}

func (e *ResponseError) Error() string {
	msg := e.APIError.Message

	if e.APIError.Target.IsSet() {
		msg += " (target: '" + *e.APIError.Target.Get() + "')"
	}

	details := make([]string, len(e.APIError.Details))
	for i, d := range e.APIError.Details {
		details[i] = d.Message
		if d.Target.IsSet() {
			details[i] += " (target: '" + *d.Target.Get() + "')"
		}
	}
	if len(details) > 0 {
		msg += "\n" + strings.Join(details, "\n")
	}

	code := strconv.Itoa(e.StatusCode)
	if e.APIError.Code != "" {
		code += "/" + e.APIError.Code
	}

	return fmt.Sprintf("(%s) %s", code, msg)
}

// CheckResponse checks an HTTP response from the provisioning API.
// It returns nil if the status code is in the 2xx range. On failure it
// tries to parse the body as the SDK's ErrorResponseApiModel shape into a
// *ResponseError; if that fails, it falls back to a friendly
// httperr-classified sentinel for 401/403, or a generic status-line error.
func CheckResponse(resp *http.Response) error {
	if resp == nil {
		return errUnknown
	}

	if c := resp.StatusCode; c >= 200 && c <= 299 {
		return nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return errUnknown
	}

	var payload ResponseError
	if err := json.Unmarshal(data, &payload); err != nil {
		if classified := httperr.ClassifyStatus(resp.StatusCode); classified != nil {
			return classified
		}
		return fmt.Errorf(
			"unexpected HTTP error from %s: %d %s",
			resp.Request.URL,
			resp.StatusCode,
			http.StatusText(resp.StatusCode),
		)
	}
	payload.StatusCode = resp.StatusCode
	return &payload
}
