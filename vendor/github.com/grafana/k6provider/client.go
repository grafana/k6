package k6provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultAuthType = "Bearer"
	buildPath       = "build"
)

// shouldRetryBuild reports whether a build request error indicates a transient
// upstream failure (502/503/504) worth retrying.
func shouldRetryBuild(err error) bool {
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	switch statusErr.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// dependency defines a dependency and its semantic version constraints
type dependency struct {
	Name        string `json:"name,omitempty"`
	Constraints string `json:"constraints,omitempty"`
}

// buildArtifact is the artifact returned by the build service (internal representation)
type buildArtifact struct {
	ID           string            `json:"id,omitempty"`
	URL          string            `json:"url,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Platform     string            `json:"platform,omitempty"`
	Checksum     string            `json:"checksum,omitempty"`
}

// buildRequest defines a request to the build service
type buildRequest struct {
	K6ModPath     string       `json:"k6_mod_path,omitempty"`
	K6Constraints string       `json:"k6,omitempty"`
	Dependencies  []dependency `json:"dependencies,omitempty"`
	Platform      string       `json:"platform,omitempty"`
}

// buildResponse defines the response for a BuildRequest
type buildResponse struct {
	Error    *WrappedError `json:"error,omitempty"`
	Artifact buildArtifact `json:"artifact"`
}

// buildClient builds custom k6 binaries via HTTP
type buildClient struct {
	srvURL   *url.URL
	auth     string
	authType string
	headers  map[string]string
	retries  int
	backoff  time.Duration
	logger   *slog.Logger
}

func newBuildServiceClient(
	urlStr, authorization, authorizationType string, headers map[string]string,
	retries int, backoff time.Duration, logger *slog.Logger,
) (*buildClient, error) {
	srvURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, NewWrappedError(ErrConfig, fmt.Errorf("invalid server URL: %w", err))
	}

	authType := authorizationType
	if authType == "" {
		authType = defaultAuthType
	}

	retries, backoff, err = resolveRetryConfig(retries, backoff)
	if err != nil {
		return nil, NewWrappedError(ErrConfig, fmt.Errorf("build service %w", err))
	}

	return &buildClient{
		srvURL:   srvURL,
		auth:     authorization,
		authType: authType,
		headers:  headers,
		retries:  retries,
		backoff:  backoff,
		logger:   logger,
	}, nil
}

func (r *buildClient) Build(
	ctx context.Context,
	platform string,
	k6ModPath string,
	k6Constraints string,
	deps []dependency,
) (buildArtifact, error) {
	req := buildRequest{
		K6ModPath:     k6ModPath,
		Platform:      platform,
		K6Constraints: k6Constraints,
		Dependencies:  deps,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return buildArtifact{}, NewWrappedError(ErrBuild, err)
	}

	resp, err := withRetry(ctx, r.retries, r.backoff, r.logger, "Build request", shouldRetryBuild,
		func() (buildResponse, error) {
			var resp buildResponse
			err := r.doRequest(ctx, buildPath, body, &resp)
			return resp, err
		})
	if err != nil {
		return buildArtifact{}, err
	}
	if resp.Error != nil {
		return buildArtifact{}, resp.Error
	}
	return resp.Artifact, nil
}

func (r *buildClient) doRequest(ctx context.Context, path string, body []byte, response any) error {
	reqURL := r.srvURL.JoinPath(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader(body))
	if err != nil {
		return NewWrappedError(ErrBuild, err)
	}
	req.Header.Set("Content-Type", "application/json")

	if r.auth != "" {
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", r.authType, r.auth))
	}

	for h, v := range r.headers {
		req.Header.Set(h, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return NewWrappedError(ErrBuild, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return NewWrappedError(ErrBuild, &StatusError{StatusCode: resp.StatusCode, Status: resp.Status})
	}

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return NewWrappedError(ErrBuild, err)
	}

	return nil
}
