// Package k6build defines a service for building k6 binaries
package k6build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
)

var ErrBuildFailed = errors.New("build failed") //nolint:revive

// Dependency defines a dependency and its semantic version constrains
type Dependency struct {
	// Name is the name of the dependency.
	Name string `json:"name,omitempty"`
	// Constraints specifies the semantic version constraints. E.g. >v0.2.0
	Constraints string `json:"constraints,omitempty"`
}

// Artifact defines the metadata of binary that satisfies a set of dependencies
// including a URL for downloading it.
type Artifact struct {
	// Unique id. Binaries satisfying the same set of dependencies have the same ID
	ID string `json:"id,omitempty"`
	// URL to fetch the artifact's binary
	URL string `json:"url,omitempty"`
	// List of dependencies that the artifact provides
	Dependencies map[string]string `json:"dependencies,omitempty"`
	// platform
	Platform string `json:"platform,omitempty"`
	// binary checksum (sha256)
	Checksum string `json:"checksum,omitempty"`
}

// String returns a text serialization of the Artifact
func (a Artifact) String() string {
	return a.toString(true, " ")
}

// Print returns a string with a pretty print of the artifact
func (a Artifact) Print() string {
	return a.toString(true, "\n")
}

// PrintSummary returns a string with a pretty print of the artifact
func (a Artifact) PrintSummary() string {
	return a.toString(false, "\n")
}

// Print returns a text serialization of the Artifact
func (a Artifact) toString(details bool, sep string) string {
	buffer := &bytes.Buffer{}
	if details {
		buffer.WriteString(fmt.Sprintf("id: %s%s", a.ID, sep))
	}
	buffer.WriteString(fmt.Sprintf("platform: %s%s", a.Platform, sep))
	for dep, version := range a.Dependencies {
		buffer.WriteString(fmt.Sprintf("%s:%q%s", dep, version, sep))
	}
	buffer.WriteString(fmt.Sprintf("checksum: %s%s", a.Checksum, sep))
	if details {
		buffer.WriteString(fmt.Sprintf("url: %s%s", a.URL, sep))
	}
	return buffer.String()
}

// BuildService defines the interface for building custom k6 binaries
type BuildService interface {
	// Build returns a k6 Artifact that satisfies a set dependencies and version constrains.
	Build(ctx context.Context, platform string, k6Constrains string, deps []Dependency) (Artifact, error)

	// Resolve returns the versions that satisfy the given dependency constrains or an error if they
	// cannot be satisfied
	Resolve(ctx context.Context, k6Constrains string, deps []Dependency) (map[string]string, error)
}
