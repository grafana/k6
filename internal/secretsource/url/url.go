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
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/secretsource"
	"golang.org/x/time/rate"
)

var (
	errInvalidConfig      = errors.New("config parameter is required in format 'config=path/to/config'")
	errMissingURLTemplate = errors.New("urlTemplate is required in config file")
	errFailedToGetSecret  = errors.New("failed to get secret")
)

// extConfig holds the configuration for URL-based secrets.
type extConfig struct {
	// URLTemplate is a URL template with {key} placeholder
	// Example: "https://api.example.com/secrets/{key}"
	URLTemplate string `json:"urlTemplate"`

	// Headers to include in the request (e.g., Authorization headers)
	Headers map[string]string `json:"headers"`

	// Method is the HTTP method to use (default: GET)
	Method null.String `json:"method"`

	// ResponsePath is a JSON path to extract the secret value from the response
	// Use dot notation for nested fields (e.g., "data.value")
	// If empty, the entire response body is treated as the secret
	ResponsePath null.String `json:"responsePath"`

	// RequestsPerMinuteLimit sets the maximum requests per minute (default: 300)
	RequestsPerMinuteLimit null.Int `json:"requestsPerMinuteLimit"`

	// RequestsBurst allows a burst of requests above the rate limit (default: 10)
	RequestsBurst null.Int `json:"requestsBurst"`

	// Timeout for HTTP requests (default: 30s)
	// Accepts duration strings like "30s", "1m", "500ms"
	Timeout types.NullDuration `json:"timeout"`

	// MaxRetries sets the maximum number of retry attempts for failed requests (default: 3)
	// Only retries on transient errors (5xx, timeouts, network errors, 429)
	// Does not retry on 4xx errors (except 429)
	MaxRetries null.Int `json:"maxRetries"`

	// RetryBackoff sets the base backoff duration for retries (default: 1s)
	// Uses exponential backoff: wait = (base ^ attempt) + jitter
	// Accepts duration strings like "1s", "500ms", "2s"
	RetryBackoff types.NullDuration `json:"retryBackoff"`
}

// newConfig creates a new extConfig instance with default values.
func newConfig() extConfig {
	return extConfig{
		Method:                 null.NewString(http.MethodGet, false),        // GET method
		ResponsePath:           null.NewString("", false),                    // Empty response path (use entire response)
		RequestsPerMinuteLimit: null.NewInt(300, false),                      // 300 requests per minute
		RequestsBurst:          null.NewInt(10, false),                       // Allow a burst of 10 requests
		Timeout:                types.NewNullDuration(30*time.Second, false), // 30 seconds timeout
		MaxRetries:             null.NewInt(3, false),                        // 3 retry attempts
		RetryBackoff:           types.NewNullDuration(1*time.Second, false),  // 1 second base backoff
	}
}

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
			limiter: newLimiter(int(config.RequestsPerMinuteLimit.Int64), int(config.RequestsBurst.Int64)),
			logger:  params.Logger,
		}, nil
	})
}

type urlSecrets struct {
	config     extConfig
	httpClient *http.Client
	limiter    limiter
	logger     logrus.FieldLogger
}

func (us *urlSecrets) Description() string {
	return "URL-based secret source"
}

func (us *urlSecrets) Get(key string) (string, error) {
	ctx := context.Background()

	if err := us.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limiter error: %w", err)
	}

	var secret string
	// MaxRetries is the number of retries, so total attempts = retries + 1
	maxAttempts := int(us.config.MaxRetries.Int64) + 1
	backoff := time.Duration(us.config.RetryBackoff.Duration)

	err := retry(ctx, maxAttempts, backoff, us.logger, func() (error, bool) {
		// Replace {key} placeholder in URL template
		url := strings.ReplaceAll(us.config.URLTemplate, "{key}", key)

		req, err := http.NewRequestWithContext(ctx, us.config.Method.String, url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err), true
		}

		// Add headers
		for k, v := range us.config.Headers {
			req.Header.Set(k, v)
		}

		response, err := us.httpClient.Do(req)
		if err != nil {
			// Network errors are retryable
			return fmt.Errorf("failed to get secret: %w", err), true
		}
		defer func() { _ = response.Body.Close() }()

		if response.StatusCode != http.StatusOK {
			statusErr := fmt.Errorf("status code %d: %w", response.StatusCode, errFailedToGetSecret)

			// Retry on server errors (5xx) and rate limiting (429)
			// Don't retry on client errors (4xx except 429)
			retryable := response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests
			return statusErr, retryable
		}

		// Extract secret value from response
		extractedSecret, err := extractSecretFromResponse(response.Body, us.config.ResponsePath.String)
		if err != nil {
			// Extraction errors are not retryable (indicates config issue)
			return fmt.Errorf("failed to extract secret: %w", err), false
		}

		secret = extractedSecret
		return nil, false
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

func extractSecretFromResponse(body io.Reader, responsePath string) (string, error) {
	const maxSecretSize = 24 * 1024

	// Limit reading to prevent memory exhaustion from large responses
	limitedReader := io.LimitReader(body, maxSecretSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if len(data) > maxSecretSize {
		return "", fmt.Errorf("secret response exceeds maximum size of %d bytes", maxSecretSize)
	}

	if responsePath == "" {
		// If no path specified, assume response body is the secret
		return string(data), nil
	}

	result := gjson.GetBytes(data, responsePath)

	if !result.Exists() {
		return "", fmt.Errorf("path %q not found in response", responsePath)
	}

	if result.Type != gjson.String {
		return "", fmt.Errorf("secret value at path %q is not a string (got %s)", responsePath, result.Type)
	}

	return result.String(), nil
}

// retry retries to execute a provided function until it succeeds or the maximum
// number of attempts is hit. It waits with exponential backoff between attempts.
// The backoff calculation is: wait = (base ^ attempt) + random jitter up to 1 second.
// The do function returns an error and a boolean indicating if the error is retryable.
func retry(
	ctx context.Context,
	attempts int,
	baseBackoff time.Duration,
	logger logrus.FieldLogger,
	do func() (error, bool),
) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			// Calculate exponential backoff: base^attempt + jitter
			wait := time.Duration(math.Pow(baseBackoff.Seconds(), float64(i))) * time.Second
			jitter := time.Duration(r.Int63n(1000)) * time.Millisecond
			wait += jitter

			logger.WithFields(logrus.Fields{
				"attempt": i + 1,
				"max":     attempts,
				"wait":    wait,
				"error":   lastErr,
			}).Debug("Retrying secret fetch after error")

			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		err, retryable := do()
		if err == nil {
			return nil
		}

		lastErr = err
		if !retryable {
			// Not a retryable error, fail immediately
			logger.WithFields(logrus.Fields{
				"attempt": i + 1,
				"error":   lastErr,
			}).Debug("Non-retryable error encountered, failing immediately")
			return lastErr
		}
	}

	logger.WithFields(logrus.Fields{
		"attempts": attempts,
		"error":    lastErr,
	}).Debug("Max retry attempts reached")

	return lastErr
}

func parseConfigArgument(configArg string) (string, error) {
	configKey, configPath, ok := strings.Cut(configArg, "=")
	if !ok || configKey != "config" {
		return "", errInvalidConfig
	}

	return configPath, nil
}

func validateURLTemplate(urlTemplate string) error {
	if urlTemplate == "" {
		return errMissingURLTemplate
	}

	// Replace {key} placeholder with a dummy value for validation
	testURL := strings.ReplaceAll(urlTemplate, "{key}", "test")
	parsedURL, err := url.Parse(testURL)
	if err != nil {
		return fmt.Errorf("urlTemplate is not a valid URL: %w", err)
	}

	// Require absolute URL with scheme
	if parsedURL.Scheme == "" {
		return errors.New("urlTemplate must be an absolute URL with a scheme (e.g., https://...)")
	}

	return nil
}

func getConfig(arg string, fs fsext.Fs) (extConfig, error) {
	// Start with default values
	config := newConfig()

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

	// Unmarshal will override defaults with user-provided values
	if err := json.Unmarshal(configData, &config); err != nil {
		return config, fmt.Errorf("failed to parse JSON config: %w", err)
	}

	// Apply defaults for fields that weren't set by the user
	if !config.Method.Valid {
		config.Method = null.StringFrom(http.MethodGet)
	}

	if !config.ResponsePath.Valid {
		config.ResponsePath = null.StringFrom("")
	}

	if !config.RequestsPerMinuteLimit.Valid {
		config.RequestsPerMinuteLimit = null.IntFrom(300)
	}

	if !config.RequestsBurst.Valid {
		config.RequestsBurst = null.IntFrom(10)
	}

	if !config.Timeout.Valid {
		config.Timeout = types.NullDurationFrom(30 * time.Second)
	}

	if !config.MaxRetries.Valid {
		config.MaxRetries = null.IntFrom(3)
	}

	if !config.RetryBackoff.Valid {
		config.RetryBackoff = types.NullDurationFrom(1 * time.Second)
	}

	// Validate required fields and value constraints
	if err := validateURLTemplate(config.URLTemplate); err != nil {
		return config, err
	}

	if config.Timeout.Duration <= 0 {
		return config, errors.New("timeout must be greater than 0")
	}

	if config.RequestsPerMinuteLimit.Int64 <= 0 {
		return config, errors.New("requestsPerMinuteLimit must be greater than 0")
	}

	if config.RequestsBurst.Int64 <= 0 {
		return config, errors.New("requestsBurst must be greater than 0")
	}

	if config.MaxRetries.Int64 < 0 {
		return config, errors.New("maxRetries must be non-negative")
	}

	if config.RetryBackoff.Duration <= 0 {
		return config, errors.New("retryBackoff must be greater than 0")
	}

	return config, nil
}
