// Package api defines the interface to a build service
package api

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/grafana/k6build"
)

var (
	// ErrBuildFailed signals the build process failed
	ErrBuildFailed = errors.New("build failed")
	// ErrCannotSatisfy signals the dependency constrains cannot be satisfied
	ErrCannotSatisfy = errors.New("cannot satisfy dependency")
	// ErrInvalidRequest signals the request could not be processed
	// due to erroneous parameters
	ErrInvalidRequest = errors.New("invalid request")
	// ErrRequestFailed signals the request failed, probably due to a network error
	ErrRequestFailed = errors.New("request failed")
	// ErrResolveFailed signals the resolve request failed
	ErrResolveFailed = errors.New("resolve failed")
)

// BuildRequest defines a request to the build service
type BuildRequest struct {
	K6Constrains string               `json:"k6,omitempty"`
	Dependencies []k6build.Dependency `json:"dependencies,omitempty"`
	Platform     string               `json:"platform,omitempty"`
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

// String returns a text serialization of the BuildResponse
func (r BuildResponse) String() string {
	buffer := &bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("artifact: %s", r.Artifact.String()))
	return buffer.String()
}

// ResolveRequest defines a request to the build service for validating if the dependency
// constrains can be satisfied
type ResolveRequest struct {
	K6Constrains string               `json:"k6,omitempty"`
	Dependencies []k6build.Dependency `json:"dependencies,omitempty"`
}

// ResolveResponse defines the response for a ResolveRequest
type ResolveResponse struct {
	// If not empty an error occurred processing the request
	// This Error can be compared to the errors defined in this package using errors.Is
	// to know the type of error, and use Unwrap to obtain its cause if available.
	Error *k6build.WrappedError `json:"error,omitempty"`
	// List of version that satisfies the dependencies
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// String returns a text serialization of the ResolveRequest
func (r ResolveRequest) String() string {
	buffer := &bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("k6: %s", r.K6Constrains))
	for _, d := range r.Dependencies {
		buffer.WriteString(fmt.Sprintf("%s:%q", d.Name, d.Constraints))
	}
	return buffer.String()
}

// String returns a text serialization of the BuildResponse
func (r ResolveResponse) String() string {
	buffer := &bytes.Buffer{}
	if r.Error != nil {
		buffer.WriteString(fmt.Sprintf("error: %s", r.Error.Error()))
	}
	for dep, version := range r.Dependencies {
		buffer.WriteString(fmt.Sprintf("%s:%q ", dep, version))
	}
	return buffer.String()
}
