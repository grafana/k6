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

	//Response *http.Response `json:"-"`
	/*ErrorData *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
	*/
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("%d %v", e.Code, e.Message)
}

var (
	AuthorizeError    = errors.New("Not allowed to upload result to Load Impact cloud")
	AuthenticateError = errors.New("Failed to authenticate with Load Impact cloud")
	UnknownError      = errors.New("An error occured talking to Load Impact cloud")
)
