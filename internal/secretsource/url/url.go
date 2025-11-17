// Package url implements a secret source that fetches secrets from generic HTTP URLs.
// This can be used as a built-in secret source or as an xk6 extension.
package url

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand" // nosemgrep: math-random-used // This is being used for retry jitter
	"net/http"
	"strings"
	"time"

	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/secretsource"
	"golang.org/x/time/rate"
)

var (
	errInvalidConfig                 = errors.New("config parameter is required in format 'config=path/to/config'")
	errMissingURLTemplate            = errors.New("urlTemplate is required in config file")
	errFailedToGetSecret             = errors.New("failed to get secret")
	errInvalidRequestsPerMinuteLimit = errors.New("requestsPerMinuteLimit must be greater than 0")
	errInvalidRequestsBurst          = errors.New("requestsBurst must be greater than 0")
	errInvalidMaxRetries             = errors.New("maxRetries must be greater than or equal to 0")
	errInvalidRetryBackoff           = errors.New("retryBackoff must be greater than 0")
	errInvalidTimeout                = errors.New("timeout must be greater than 0")
)

// retryableError wraps an error with HTTP status code information to determine if it should be retried.
type retryableError struct {
	err        error
	statusCode int
}

func (re *retryableError) Error() string {
	if re.statusCode > 0 {
		return fmt.Sprintf("HTTP %d: %v", re.statusCode, re.err)
	}
	return re.err.Error()
}

func (re *retryableError) Unwrap() error {
	return re.err
}

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

	// Timeout for HTTP requests (defaults to 30s)
	// Accepts duration strings like "30s", "1m", "500ms"
	Timeout types.NullDuration `json:"timeout"`

	// MaxRetries sets the maximum number of retry attempts for failed requests
	// Only retries on transient errors (5xx, timeouts, network errors, 429)
	// Does not retry on 4xx errors (except 429)
	MaxRetries *int `json:"maxRetries"`

	// RetryBackoff sets the base backoff duration for retries (defaults to 1s)
	// Uses exponential backoff: wait = (base ^ attempt) + jitter
	// Accepts duration strings like "1s", "500ms", "2s"
	RetryBackoff types.NullDuration `json:"retryBackoff"`
}

const (
	// The rate limiter replenishes tokens in the bucket once every 200 ms
	// (5 per second) and allows a burst of 10 requests firing faster than
	// that. If the client keeps making requests at the rapid pace, they
	// will be slowed down. This allows a client to ask for a bunch of
	// secrets at the start of a script, and then it slows it down to a
	// reasonable pace.
	defaultRequestsPerMinuteLimit = 300              // 300 requests per minute is one request every 200 ms
	defaultRequestsBurst          = 10               // Allow a burst of 10 requests
	defaultTimeout                = 30 * time.Second // 30 seconds timeout
	defaultMaxRetries             = 3                // 3 retry attempts for transient failures
	defaultRetryBackoff           = 1 * time.Second  // 1 second base for exponential backoff
)

//nolint:gochecknoinits // This is how k6 secret source registration works.
func init() {
	secretsource.RegisterExtension("url", func(params secretsource.Params) (secretsource.Source, error) {
		config, err := getConfig(params.ConfigArgument, params.FS)
		if err != nil {
			return nil, fmt.Errorf("missing or invalid config: %w", err)
		}

		return &urlSecrets{
			config: config,
			httpClient: &http.Client{
				Timeout: time.Duration(config.Timeout.Duration),
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

	var secret string
	maxAttempts := *us.config.MaxRetries + 1 // MaxRetries is the number of retries, so total attempts = retries + 1
	backoff := time.Duration(us.config.RetryBackoff.Duration)

	err := retry(ctx, maxAttempts, backoff, func() error {
		// Replace {key} placeholder in URL template
		url := strings.ReplaceAll(us.config.URLTemplate, "{key}", key)

		method := us.config.Method
		if method == "" {
			method = http.MethodGet
		}

		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return &retryableError{err: fmt.Errorf("failed to create request: %w", err), statusCode: 0}
		}

		// Add headers
		for k, v := range us.config.Headers {
			req.Header.Set(k, v)
		}

		response, err := us.httpClient.Do(req)
		if err != nil {
			// Network errors are retryable
			return &retryableError{err: fmt.Errorf("failed to get secret: %w", err), statusCode: 0}
		}
		defer func() { _ = response.Body.Close() }()

		if response.StatusCode != http.StatusOK {
			statusErr := fmt.Errorf("status code %d: %w", response.StatusCode, errFailedToGetSecret)

			// Check if this is a retryable status code
			if isRetryableError(response.StatusCode, nil) {
				return &retryableError{err: statusErr, statusCode: response.StatusCode}
			}

			// Non-retryable error (4xx except 429)
			return statusErr
		}

		body, err := io.ReadAll(response.Body)
		if err != nil {
			return &retryableError{err: fmt.Errorf("failed to read response: %w", err), statusCode: 0}
		}

		// Extract secret value from response
		extractedSecret, err := extractSecretFromResponse(body, us.config.ResponsePath)
		if err != nil {
			// Extraction errors are not retryable (indicates config issue)
			return fmt.Errorf("failed to extract secret: %w", err)
		}

		secret = extractedSecret
		return nil
	})
	if err != nil {
		return "", err
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

// isRetryableError determines if an HTTP status code or error should trigger a retry.
// Retries on:
// - Network errors (connection failures, timeouts)
// - 5xx server errors (server-side issues)
// - 429 Too Many Requests (rate limiting)
// Does NOT retry on:
// - 4xx client errors (except 429) - these indicate issues with the request itself
func isRetryableError(statusCode int, err error) bool {
	// Network errors should be retried
	if err != nil {
		return true
	}

	// Retry on server errors and rate limiting
	if statusCode >= 500 || statusCode == http.StatusTooManyRequests {
		return true
	}

	return false
}

// retry retries to execute a provided function until it succeeds or the maximum
// number of attempts is hit. It waits with exponential backoff between attempts.
// The backoff calculation is: wait = (base ^ attempt) + random jitter up to 1 second.
// Only retries errors that are of type *retryableError.
func retry(ctx context.Context, attempts int, baseBackoff time.Duration, do func() error) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			// Calculate exponential backoff: base^attempt + jitter
			wait := time.Duration(math.Pow(baseBackoff.Seconds(), float64(i))) * time.Second
			jitter := time.Duration(r.Int63n(1000)) * time.Millisecond
			wait += jitter

			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		lastErr = do()
		if lastErr == nil {
			return nil
		}

		// Check if this error should be retried
		var retryErr *retryableError
		if !errors.As(lastErr, &retryErr) {
			// Not a retryable error, fail immediately
			return lastErr
		}

		// If this is the last attempt, return the error
		if i == attempts-1 {
			return lastErr
		}
	}

	return lastErr
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

	if !config.Timeout.Valid {
		config.Timeout = types.NullDurationFrom(defaultTimeout)
	}

	if config.Timeout.Duration <= 0 {
		return config, errInvalidTimeout
	}

	if *config.RequestsPerMinuteLimit <= 0 {
		return config, errInvalidRequestsPerMinuteLimit
	}

	if *config.RequestsBurst <= 0 {
		return config, errInvalidRequestsBurst
	}

	if config.MaxRetries == nil {
		maxRetries := defaultMaxRetries
		config.MaxRetries = &maxRetries
	}

	if *config.MaxRetries < 0 {
		return config, errInvalidMaxRetries
	}

	if !config.RetryBackoff.Valid {
		config.RetryBackoff = types.NullDurationFrom(defaultRetryBackoff)
	}

	if config.RetryBackoff.Duration <= 0 {
		return config, errInvalidRetryBackoff
	}

	return config, nil
}
