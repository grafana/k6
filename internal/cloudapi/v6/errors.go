package cloudapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
)

var (
	errNotAuthorized    = errors.New("not allowed to upload result to k6 Cloud")
	errNotAuthenticated = errors.New("failed to authenticate with k6 Cloud")
	errUnknown          = errors.New("an error occurred communicating with k6 Cloud")
)

// ResponseError represents an error cause by talking to the API
type ResponseError struct {
	Response *http.Response        `json:"-"`
	APIError k6cloud.ErrorApiModel `json:"error"`
}

func (e ResponseError) Error() string {
	err := e.APIError
	msg := err.Message

	if err.Target.IsSet() {
		msg += " (target: '" + *err.Target.Get() + "')"
	}

	details := make([]string, len(err.Details))
	for i, v := range err.Details {
		details[i] = v.Message
		if v.Target.IsSet() {
			details[i] += " (target: '" + *v.Target.Get() + "')"
		}
	}

	if len(details) > 0 {
		msg += "\n" + strings.Join(details, "\n")
	}

	var code string
	switch {
	case err.Code != "" && e.Response != nil:
		code = fmt.Sprintf("%d/%s", e.Response.StatusCode, err.Code)
	case e.Response != nil:
		code = fmt.Sprintf("%d", e.Response.StatusCode)
	case err.Code != "":
		code = err.Code
	}

	if len(code) > 0 {
		msg = fmt.Sprintf("(%s) %s", code, msg)
	}

	return msg
}
