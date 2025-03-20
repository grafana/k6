// Package api defines the interface to a build service
package api

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/grafana/k6build"
)

var (
	// ErrInvalidRequest signals the request could not be processed
	// due to erroneous parameters
	ErrInvalidRequest = errors.New("invalid request")
	// ErrRequestFailed signals the request failed, probably due to a network error
	ErrRequestFailed = errors.New("request failed")
	// ErrBuildFailed signals the build process failed
	ErrBuildFailed = errors.New("build failed")
)

// BuildRequest defines a request to the build service
type BuildRequest struct {
	K6Constrains string               `json:"k6,omitempty"`
	Dependencies []k6build.Dependency `json:"dependencies,omitempty"`
	Platform     string               `json:"platform,omitempty"`
}

// String returns a text serialization of the BuildRequest
func (r BuildRequest) String() string {
	buffer := &bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("platform: %s", r.Platform))
	buffer.WriteString(fmt.Sprintf("k6: %s", r.K6Constrains))
	for _, d := range r.Dependencies {
		buffer.WriteString(fmt.Sprintf("%s:%q", d.Name, d.Constraints))
	}
	return buffer.String()
}

// BuildResponse defines the response for a BuildRequest
type BuildResponse struct {
	// If not empty an error occurred processing the request
	// This Error can be compared to the errors defined in this package using errors.Is
	// to know the type of error, and use Unwrap to obtain its cause if available.
	Error *k6build.WrappedError `json:"error,omitempty"`
	// Artifact metadata. If an error occurred, content is undefined
	Artifact k6build.Artifact `json:"artifact,omitempty"`
}
