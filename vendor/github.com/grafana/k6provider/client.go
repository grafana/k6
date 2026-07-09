package k6provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const (
	defaultAuthType = "Bearer"
	buildPath       = "build"
)

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
}

func newBuildServiceClient(
	urlStr, authorization, authorizationType string, headers map[string]string,
) (*buildClient, error) {
	srvURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, NewWrappedError(ErrConfig, fmt.Errorf("invalid server URL: %w", err))
	}

	authType := authorizationType
	if authType == "" {
		authType = defaultAuthType
	}

	return &buildClient{
		srvURL:   srvURL,
		auth:     authorization,
		authType: authType,
		headers:  headers,
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

	var resp buildResponse
	if err := r.doRequest(ctx, buildPath, &req, &resp); err != nil {
		return buildArtifact{}, err
	}
	if resp.Error != nil {
		return buildArtifact{}, resp.Error
	}
	return resp.Artifact, nil
}

func (r *buildClient) doRequest(ctx context.Context, path string, request, response any) error {
	marshaled := &bytes.Buffer{}
	if err := json.NewEncoder(marshaled).Encode(request); err != nil {
		return NewWrappedError(ErrBuild, err)
	}

	reqURL := r.srvURL.JoinPath(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), marshaled)
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

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: URL is from build service config, not user input
	if err != nil {
		return NewWrappedError(ErrBuild, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return NewWrappedError(ErrBuild, fmt.Errorf("status %s", resp.Status))
	}

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return NewWrappedError(ErrBuild, err)
	}

	return nil
}
