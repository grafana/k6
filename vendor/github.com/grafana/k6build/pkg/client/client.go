// Package client implements a client for a build service
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/grafana/k6build"
	"github.com/grafana/k6build/pkg/api"
)

// ErrInvalidConfiguration signals an error in the configuration
var ErrInvalidConfiguration = errors.New("invalid configuration")

const (
	defaultAuthType = "Bearer"
	buildPath       = "build"
	resolvePath     = "resolve"

	// DefaultRetries number of retries for requests
	DefaultRetries = 3
	// DefaultBackoff initial backoff time between retries. It is incremented exponentially between retries.
	DefaultBackoff = 1 * time.Second
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
	// Retries number of retries for requests. Default to 3
	Retries int
	// Backoff initial backoff time between retries. Default to 1s
	// It is incremented exponentially between retries: 1s, 2s, 4s...
	Backoff time.Duration
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
		retries:  config.Retries,
		backoff:  config.Backoff,
	}, nil
}

// BuildClient defines a client of a build service
type BuildClient struct {
	srvURL   *url.URL
	authType string
	auth     string
	headers  map[string]string
	client   *http.Client
	retries  int
	backoff  time.Duration
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

	buildResponse := api.BuildResponse{}

	err := r.doRequest(ctx, buildPath, &buildRequest, &buildResponse)
	if err != nil {
		return k6build.Artifact{}, err
	}

	if buildResponse.Error != nil {
		return k6build.Artifact{}, buildResponse.Error
	}

	return buildResponse.Artifact, nil
}

// Resolve returns the versions that satisfy the given dependencies or an error if they cannot be
// satisfied
func (r *BuildClient) Resolve(
	ctx context.Context,
	k6Constrains string,
	deps []k6build.Dependency,
) (map[string]string, error) {
	resolveRequest := api.ResolveRequest{
		K6Constrains: k6Constrains,
		Dependencies: deps,
	}

	resolveResponse := api.ResolveResponse{}

	err := r.doRequest(ctx, resolvePath, &resolveRequest, &resolveResponse)
	if err != nil {
		return nil, err
	}

	if resolveResponse.Error != nil {
		return nil, resolveResponse.Error
	}

	return resolveResponse.Dependencies, nil
}

func (r *BuildClient) doRequest(ctx context.Context, path string, request any, response any) error {
	marshaled := &bytes.Buffer{}
	err := json.NewEncoder(marshaled).Encode(request)
	if err != nil {
		return k6build.NewWrappedError(api.ErrInvalidRequest, err)
	}

	reqURL := r.srvURL.JoinPath(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), marshaled)
	if err != nil {
		return k6build.NewWrappedError(api.ErrRequestFailed, err)
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

	var (
		resp    *http.Response
		backoff = r.backoff
		retries = r.retries
	)

	if retries == 0 {
		retries = DefaultRetries
	}

	if backoff == 0 {
		backoff = DefaultBackoff
	}

	// preserve body for retries
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return k6build.NewWrappedError(api.ErrRequestFailed, err)
	}

	// close the original body, we don't need it anymore
	err = req.Body.Close()
	if err != nil {
		return k6build.NewWrappedError(api.ErrRequestFailed, err)
	}

	// try at least once
	for {
		req.Body = io.NopCloser(bytes.NewReader(body)) // reset the body
		resp, err = r.client.Do(req)

		if retries == 0 || !shouldRetry(err, resp) {
			break
		}

		time.Sleep(backoff)

		// increase backoff exponentially for next retry
		backoff *= 2
		retries--
	}

	if err != nil {
		return k6build.NewWrappedError(api.ErrRequestFailed, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return k6build.NewWrappedError(api.ErrRequestFailed, errors.New(resp.Status))
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return k6build.NewWrappedError(api.ErrRequestFailed, err)
	}

	return nil
}

// shouldRetry returns true if the error or response indicates that the request should be retried
func shouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		if errors.Is(err, io.EOF) { // assuming EOF is due to connection interrupted by network error
			return true
		}

		var ne net.Error
		if errors.As(err, &ne) {
			return ne.Timeout()
		}

		return false
	}

	if resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusBadGateway ||
		resp.StatusCode == http.StatusGatewayTimeout {
		return true
	}

	return false
}
