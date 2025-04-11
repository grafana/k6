// Package client implements a client for a build service
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/grafana/k6build"
	"github.com/grafana/k6build/pkg/api"
)

// ErrInvalidConfiguration signals an error in the configuration
var ErrInvalidConfiguration = errors.New("invalid configuration")

const (
	defaultAuthType = "Bearer"
)

// BuildServiceClientConfig defines the configuration for accessing a remote build service
type BuildServiceClientConfig struct {
	// URL to build service
	URL string
	// Authorization credentials passed in the Authorization: <type> <credentials> header
	// See AuthorizationType
	Authorization string
	// AuthorizationType type of credentials in the Authorization: <type> <credentials> header
	// For example, "Bearer", "Token", "Basic". Defaults to "Bearer"
	AuthorizationType string
	// Headers custom request headers
	Headers map[string]string
	// HTTPClient custom http client
	HTTPClient *http.Client
}

// NewBuildServiceClient returns a new client for a remote build service
func NewBuildServiceClient(config BuildServiceClientConfig) (k6build.BuildService, error) {
	if config.URL == "" {
		return nil, ErrInvalidConfiguration
	}

	srvURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid server %w", err)
	}

	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &BuildClient{
		srvURL:   srvURL,
		auth:     config.Authorization,
		authType: config.AuthorizationType,
		headers:  config.Headers,
		client:   client,
	}, nil
}

// BuildClient defines a client of a build service
type BuildClient struct {
	srvURL   *url.URL
	authType string
	auth     string
	headers  map[string]string
	client   *http.Client
}

// Build request building an artifact to a build service
// The build service is expected to return a k6build.Artifact
// In case of error, the returned error is expected to match any of the errors
// defined in the api package and calling errors.Unwrap(err) will provide
// the cause, if available.
func (r *BuildClient) Build(
	ctx context.Context,
	platform string,
	k6Constrains string,
	deps []k6build.Dependency,
) (k6build.Artifact, error) {
	buildRequest := api.BuildRequest{
		Platform:     platform,
		K6Constrains: k6Constrains,
		Dependencies: deps,
	}
	marshaled := &bytes.Buffer{}
	err := json.NewEncoder(marshaled).Encode(buildRequest)
	if err != nil {
		return k6build.Artifact{}, k6build.NewWrappedError(api.ErrInvalidRequest, err)
	}

	reqURL := r.srvURL.JoinPath("build")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), marshaled)
	if err != nil {
		return k6build.Artifact{}, k6build.NewWrappedError(api.ErrRequestFailed, err)
	}
	req.Header.Add("Content-Type", "application/json")

	// add authorization header "Authorization: <type> <auth>"
	if r.auth != "" {
		authType := r.authType
		if authType == "" {
			authType = defaultAuthType
		}
		req.Header.Add("Authorization", fmt.Sprintf("%s %s", authType, r.auth))
	}

	// add custom headers
	for h, v := range r.headers {
		req.Header.Add(h, v)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return k6build.Artifact{}, k6build.NewWrappedError(api.ErrRequestFailed, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return k6build.Artifact{}, k6build.NewWrappedError(api.ErrRequestFailed, errors.New(resp.Status))
	}

	buildResponse := api.BuildResponse{}
	err = json.NewDecoder(resp.Body).Decode(&buildResponse)
	if err != nil {
		return k6build.Artifact{}, k6build.NewWrappedError(api.ErrRequestFailed, err)
	}

	if buildResponse.Error != nil {
		return k6build.Artifact{}, buildResponse.Error
	}

	return buildResponse.Artifact, nil
}
