// Package url implements a secret source that fetches secrets from generic HTTP URLs.
// This can be used as a built-in secret source or as an xk6 extension.
package url

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/secretsource"
	"golang.org/x/time/rate"
)

var (
	errInvalidConfig                 = errors.New("config parameter is required in format 'config=path/to/config'")
	errMissingURLTemplate            = errors.New("urlTemplate is required in config file")
	errFailedToGetSecret             = errors.New("failed to get secret")
	errInvalidRequestsPerMinuteLimit = errors.New("requestsPerMinuteLimit must be greater than 0")
	errInvalidRequestsBurst          = errors.New("requestsBurst must be greater than 0")
)

// extConfig holds the configuration for URL-based secrets.
type extConfig struct {
	// URLTemplate is a URL template with {key} placeholder
	// Example: "https://api.example.com/secrets/{key}"
	URLTemplate string `json:"urlTemplate"`

	// Headers to include in the request (e.g., Authorization headers)
	Headers map[string]string `json:"headers"`

	// Method is the HTTP method to use (defaults to GET)
	Method string `json:"method"`

	// ResponsePath is a JSON path to extract the secret value from the response
	// Use dot notation for nested fields (e.g., "data.value")
	// If empty, the entire response body is treated as the secret
	ResponsePath string `json:"responsePath"`

	// RequestsPerMinuteLimit sets the maximum requests per minute
	RequestsPerMinuteLimit *int `json:"requestsPerMinuteLimit"`

	// RequestsBurst allows a burst of requests above the rate limit
	RequestsBurst *int `json:"requestsBurst"`

	// Timeout for HTTP requests in seconds (defaults to 30)
	TimeoutSeconds *int `json:"timeoutSeconds"`
}

const (
	// The rate limiter replenishes tokens in the bucket once every 200 ms
	// (5 per second) and allows a burst of 10 requests firing faster than
	// that. If the client keeps making requests at the rapid pace, they
	// will be slowed down. This allows a client to ask for a bunch of
	// secrets at the start of a script, and then it slows it down to a
	// reasonable pace.
	defaultRequestsPerMinuteLimit = 300 // 300 requests per minute is one request every 200 ms
	defaultRequestsBurst          = 10  // Allow a burst of 10 requests
	defaultTimeoutSeconds         = 30  // 30 seconds timeout
)

//nolint:gochecknoinits // This is how k6 secret source registration works.
func init() {
	secretsource.RegisterExtension("url", func(params secretsource.Params) (secretsource.Source, error) {
		config, err := getConfig(params.ConfigArgument, params.FS)
		if err != nil {
			return nil, fmt.Errorf("missing or invalid config: %w", err)
		}

		timeout := time.Duration(*config.TimeoutSeconds) * time.Second

		return &urlSecrets{
			config: config,
			httpClient: &http.Client{
				Timeout: timeout,
			},
			limiter: newLimiter(*config.RequestsPerMinuteLimit, *config.RequestsBurst),
		}, nil
	})
}

type urlSecrets struct {
	config     extConfig
	httpClient *http.Client
	limiter    limiter
}

func (us *urlSecrets) Description() string {
	return fmt.Sprintf("URL-based secret source from %s", us.config.URLTemplate)
}

func (us *urlSecrets) Get(key string) (string, error) {
	ctx := context.Background()

	if err := us.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limiter error: %w", err)
	}

	// Replace {key} placeholder in URL template
	url := strings.ReplaceAll(us.config.URLTemplate, "{key}", key)

	method := us.config.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for k, v := range us.config.Headers {
		req.Header.Set(k, v)
	}

	response, err := us.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code %d: %w", response.StatusCode, errFailedToGetSecret)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Extract secret value from response
	secret, err := extractSecretFromResponse(body, us.config.ResponsePath)
	if err != nil {
		return "", fmt.Errorf("failed to extract secret: %w", err)
	}

	return secret, nil
}

type limiter interface {
	Wait(ctx context.Context) error
}

func newLimiter(requestsPerMinuteLimit, requestsBurst int) *rate.Limiter {
	// The calculation below looks wrong because it seems like it's giving
	// n min²/req, but the first number is actually time unit/min, so the
	// units of the result are time unit/req, which is correct because it's
	// the interval of time after which a new token is replenished. In
	// other words, the units are time unit/token.
	tokenReplenishInterval := time.Minute / time.Duration(requestsPerMinuteLimit)

	return rate.NewLimiter(rate.Every(tokenReplenishInterval), requestsBurst)
}

func extractSecretFromResponse(body []byte, responsePath string) (string, error) {
	if responsePath == "" {
		// If no path specified, assume response body is the secret
		return string(body), nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to decode JSON response: %w", err)
	}

	// Simple JSON path traversal for nested keys like "data.value"
	parts := strings.Split(responsePath, ".")
	current := interface{}(data)

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			val, exists := m[part]
			if !exists {
				return "", fmt.Errorf("path component %q not found in response", part)
			}
			current = val
		} else {
			return "", fmt.Errorf("cannot traverse path at %q: not a JSON object", part)
		}
	}

	if secret, ok := current.(string); ok {
		return secret, nil
	}

	return "", fmt.Errorf("secret value at path %q is not a string", responsePath)
}

func parseConfigArgument(configArg string) (string, error) {
	configKey, configPath, ok := strings.Cut(configArg, "=")
	if !ok || configKey != "config" {
		return "", errInvalidConfig
	}

	return configPath, nil
}

func getConfig(arg string, fs fsext.Fs) (extConfig, error) {
	var config extConfig

	// Parse the ConfigArgument to get the config file path
	configPath, err := parseConfigArgument(arg)
	if err != nil {
		return config, err
	}

	file, err := fs.Open(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to open config file: %w", err)
	}
	defer func() { _ = file.Close() }()

	configData, err := io.ReadAll(file)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		return config, fmt.Errorf("failed to parse JSON config: %w", err)
	}

	if config.URLTemplate == "" {
		return config, errMissingURLTemplate
	}

	if config.RequestsPerMinuteLimit == nil {
		requestsPerMinuteLimit := defaultRequestsPerMinuteLimit
		config.RequestsPerMinuteLimit = &requestsPerMinuteLimit
	}

	if config.RequestsBurst == nil {
		requestsBurst := defaultRequestsBurst
		config.RequestsBurst = &requestsBurst
	}

	if config.TimeoutSeconds == nil {
		timeoutSeconds := defaultTimeoutSeconds
		config.TimeoutSeconds = &timeoutSeconds
	}

	if *config.RequestsPerMinuteLimit <= 0 {
		return config, errInvalidRequestsPerMinuteLimit
	}

	if *config.RequestsBurst <= 0 {
		return config, errInvalidRequestsBurst
	}

	return config, nil
}
