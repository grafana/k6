// Package cloud implements a secret source that fetches secrets from Grafana Cloud k6.
// This is a thin wrapper around the URL secret source that automatically configures
// itself from the cloud API response.
package cloud

import (
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"

	"go.k6.io/k6/v2/internal/secretsource/url"
	"go.k6.io/k6/v2/secretsource"
)

// Config holds the configuration for accessing cloud secrets.
type Config struct {
	// Token is the ephemeral token for accessing secrets during test execution.
	Token string

	// Endpoint is the URL template with {key} placeholder.
	Endpoint string

	// ResponsePath is the JSON path to extract the secret value from the response.
	ResponsePath string
}

// New creates a SecretSource with its own per-instance config pointer.
// Exported so createSecretSources can pre-register it and later call SetConfig
// once the cloud API response is available (before any VU goroutine calls Get).
func New(params secretsource.Params) (*SecretSource, error) {
	return &SecretSource{
		params:    params,
		configPtr: &atomic.Pointer[Config]{},
	}, nil
}

//nolint:gochecknoinits // This is how k6 secret source registration works.
func init() {
	secretsource.RegisterExtension("cloud", func(params secretsource.Params) (secretsource.Source, error) {
		return New(params)
	})
}

// SecretSource wraps the URL secret source for cloud-based secrets.
// It is automatically registered for 'k6 cloud run --local-execution' (unless
// --no-cloud-secrets is passed) and for the PLZ operator path when
// K6_CLOUD_SECRETS_TOKEN is set. Configuration arrives via SetConfig — either
// from the CreateTestRun API response (createCloudTest) or directly from env
// vars — before the first Get() call.
//
// The URL source is rebuilt whenever configPtr changes (i.e. a new test run starts with
// different credentials). This supports sequential test runs in the same process — each run
// calls SetConfig with its own token/endpoint, and the URL source is re-initialized.
// A mutex serialises concurrent Get() calls during initialisation.
type SecretSource struct {
	params    secretsource.Params
	configPtr *atomic.Pointer[Config]
	mu        sync.Mutex
	activeCfg *Config // the Config pointer that urlSource was built from
	urlSource secretsource.Source
	initErr   error
}

// SetConfig stores the cloud secrets configuration. Called from createCloudTest once the
// CreateTestRun API response is available, before any VU goroutine can call Get().
func (cs *SecretSource) SetConfig(c *Config) {
	cs.configPtr.Store(c)
}

// Description returns a description of this secret source.
func (cs *SecretSource) Description() string {
	return "Grafana Cloud k6 secret source"
}

// ensureInitialized builds (or rebuilds) the URL source from configPtr.
func (cs *SecretSource) ensureInitialized() (secretsource.Source, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	current := cs.configPtr.Load()

	// Re-use the cached source if the config pointer is unchanged.
	if cs.activeCfg == current && (cs.urlSource != nil || cs.initErr != nil) {
		return cs.urlSource, cs.initErr
	}

	// (Re-)initialize for the new config.
	cs.activeCfg = current
	cs.urlSource = nil
	cs.initErr = nil

	if current == nil {
		cs.initErr = errors.New("cloud secrets not configured: no secrets configuration available. " +
			"Make sure you're using 'k6 cloud run --local-execution' and the cloud API " +
			"returned secrets configuration")
		return nil, cs.initErr
	}

	if current.Token == "" {
		cs.initErr = errors.New("cloud secrets not configured: token not set")
		return nil, cs.initErr
	}

	if current.Endpoint == "" {
		cs.initErr = errors.New("cloud secrets not configured: endpoint not set")
		return nil, cs.initErr
	}

	extra := 2 // always: URL template + Authorization header
	if current.ResponsePath != "" {
		extra = 3
	}
	envCopy := make(map[string]string, len(cs.params.Environment)+extra)
	maps.Copy(envCopy, cs.params.Environment)
	envCopy["K6_SECRET_SOURCE_URL_URL_TEMPLATE"] = current.Endpoint
	envCopy["K6_SECRET_SOURCE_URL_HEADER_AUTHORIZATION"] = "Bearer " + current.Token
	if current.ResponsePath != "" {
		envCopy["K6_SECRET_SOURCE_URL_RESPONSE_PATH"] = current.ResponsePath
	}

	p := cs.params
	p.Environment = envCopy
	cs.urlSource, cs.initErr = url.New(p)
	if cs.initErr != nil {
		cs.initErr = fmt.Errorf("failed to initialize cloud secret source: %w", cs.initErr)
		return nil, cs.initErr
	}
	return cs.urlSource, nil
}

// Get retrieves a secret from the cloud by delegating to the URL secret source.
func (cs *SecretSource) Get(key string) (string, error) {
	src, err := cs.ensureInitialized()
	if err != nil {
		return "", err
	}
	return src.Get(key)
}
