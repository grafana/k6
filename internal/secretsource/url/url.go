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
	errMissingURLTemplate = errors.New("urlTemplate is required")
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
		Method:                 null.StringFrom(http.MethodGet),          // GET method
		ResponsePath:           null.StringFrom(""),                      // Empty response path (use entire response)
		RequestsPerMinuteLimit: null.IntFrom(300),                        // 300 requests per minute
		RequestsBurst:          null.IntFrom(10),                         // Allow a burst of 10 requests
		Timeout:                types.NullDurationFrom(30 * time.Second), // 30 seconds timeout
		MaxRetries:             null.IntFrom(3),                          // 3 retry attempts
		RetryBackoff:           types.NullDurationFrom(1 * time.Second),  // 1 second base backoff
	}
}

func (c extConfig) Apply(cfg extConfig) extConfig {
	result := c

	if cfg.URLTemplate != "" {
		result.URLTemplate = cfg.URLTemplate
	}

	if cfg.Headers != nil {
		result.Headers = cfg.Headers
	}

	if cfg.Method.Valid {
		result.Method = cfg.Method
	}

	if cfg.ResponsePath.Valid {
		result.ResponsePath = cfg.ResponsePath
	}

	if cfg.RequestsPerMinuteLimit.Valid {
		result.RequestsPerMinuteLimit = cfg.RequestsPerMinuteLimit
	}

	if cfg.RequestsBurst.Valid {
		result.RequestsBurst = cfg.RequestsBurst
	}

	if cfg.Timeout.Valid {
		result.Timeout = cfg.Timeout
	}

	if cfg.MaxRetries.Valid {
		result.MaxRetries = cfg.MaxRetries
	}

	if cfg.RetryBackoff.Valid {
		result.RetryBackoff = cfg.RetryBackoff
	}

	return result
}

//nolint:gochecknoinits // This is how k6 secret source registration works.
func init() {
	secretsource.RegisterExtension("url", func(params secretsource.Params) (secretsource.Source, error) {
		config, err := getConfig(params.ConfigArgument, params.FS, params.Environment)
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

		response, err := us.httpClient.Do(req) //nolint:gosec
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

func parseInlineConfig(configArg string, fs fsext.Fs) (extConfig, error) {
	var fileCfg extConfig
	var inlineCfg extConfig

	// Split by comma to parse key=value pairs
	parts := strings.Split(configArg, ",")
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return extConfig{}, fmt.Errorf("invalid config format %q, expected key=value", part)
		}

		if key == "config" {
			// Load from file
			var err error
			fileCfg, err = loadConfigFromFile(value, fs)
			if err != nil {
				return extConfig{}, err
			}
			continue
		}

		if err := parseInlineConfigOption(key, value, &inlineCfg); err != nil {
			return extConfig{}, err
		}
	}

	// Apply configs in order: file -> inline
	// This allows inline config to override file config
	config := fileCfg.Apply(inlineCfg)

	return config, nil
}

func parseInlineConfigOption(key, value string, cfg *extConfig) error {
	switch key {
	case "urlTemplate":
		cfg.URLTemplate = value
	case "method":
		cfg.Method = null.StringFrom(value)
	case "responsePath":
		cfg.ResponsePath = null.StringFrom(value)
	case "timeout":
		return parseDurationOption(value, &cfg.Timeout, "timeout")
	case "requestsPerMinuteLimit":
		return parseIntOption(value, &cfg.RequestsPerMinuteLimit, "requestsPerMinuteLimit")
	case "requestsBurst":
		return parseIntOption(value, &cfg.RequestsBurst, "requestsBurst")
	case "maxRetries":
		return parseIntOption(value, &cfg.MaxRetries, "maxRetries")
	case "retryBackoff":
		return parseDurationOption(value, &cfg.RetryBackoff, "retryBackoff")
	default:
		return parseHeaderOption(key, value, cfg)
	}
	return nil
}

func parseIntOption(value string, target *null.Int, optionName string) error {
	var intValue int64
	_, err := fmt.Sscanf(value, "%d", &intValue)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", optionName, err)
	}
	*target = null.IntFrom(intValue)
	return nil
}

func parseDurationOption(value string, target *types.NullDuration, optionName string) error {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", optionName, err)
	}
	*target = types.NullDurationFrom(duration)
	return nil
}

func parseHeaderOption(key, value string, cfg *extConfig) error {
	if !strings.HasPrefix(key, "headers.") {
		return fmt.Errorf("unknown configuration key: %q", key)
	}

	headerName := strings.TrimPrefix(key, "headers.")
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}
	cfg.Headers[headerName] = value
	return nil
}

// parseEnvConfig reads configuration from environment variables.
// It looks for variables with the prefix K6_SECRET_SOURCE_URL_.
// Example environment variables:
//   - K6_SECRET_SOURCE_URL_URL_TEMPLATE
//   - K6_SECRET_SOURCE_URL_HEADER_AUTHORIZATION
//   - K6_SECRET_SOURCE_URL_METHOD
//   - K6_SECRET_SOURCE_URL_RESPONSE_PATH
//   - K6_SECRET_SOURCE_URL_TIMEOUT
//   - K6_SECRET_SOURCE_URL_MAX_RETRIES
//   - K6_SECRET_SOURCE_URL_RETRY_BACKOFF
//   - K6_SECRET_SOURCE_URL_REQUESTS_PER_MINUTE_LIMIT
//   - K6_SECRET_SOURCE_URL_REQUESTS_BURST
//
//nolint:gocognit // Function parses multiple env vars, each adding to complexity
func parseEnvConfig(env map[string]string) (extConfig, error) {
	var cfg extConfig

	// Read URL template (required)
	if urlTemplate, ok := env["K6_SECRET_SOURCE_URL_URL_TEMPLATE"]; ok && urlTemplate != "" {
		cfg.URLTemplate = urlTemplate
	}

	// Read method
	if method, ok := env["K6_SECRET_SOURCE_URL_METHOD"]; ok && method != "" {
		cfg.Method = null.StringFrom(method)
	}

	// Read response path
	if responsePath, ok := env["K6_SECRET_SOURCE_URL_RESPONSE_PATH"]; ok && responsePath != "" {
		cfg.ResponsePath = null.StringFrom(responsePath)
	}

	// Read timeout
	if timeoutStr, ok := env["K6_SECRET_SOURCE_URL_TIMEOUT"]; ok && timeoutStr != "" {
		if err := parseDurationOption(timeoutStr, &cfg.Timeout, "timeout"); err != nil {
			return extConfig{}, err
		}
	}

	// Read max retries
	if maxRetriesStr, ok := env["K6_SECRET_SOURCE_URL_MAX_RETRIES"]; ok && maxRetriesStr != "" {
		if err := parseIntOption(maxRetriesStr, &cfg.MaxRetries, "maxRetries"); err != nil {
			return extConfig{}, err
		}
	}

	// Read retry backoff
	if retryBackoffStr, ok := env["K6_SECRET_SOURCE_URL_RETRY_BACKOFF"]; ok && retryBackoffStr != "" {
		if err := parseDurationOption(retryBackoffStr, &cfg.RetryBackoff, "retryBackoff"); err != nil {
			return extConfig{}, err
		}
	}

	// Read requests per minute limit
	if rpmStr, ok := env["K6_SECRET_SOURCE_URL_REQUESTS_PER_MINUTE_LIMIT"]; ok && rpmStr != "" {
		if err := parseIntOption(rpmStr, &cfg.RequestsPerMinuteLimit, "requestsPerMinuteLimit"); err != nil {
			return extConfig{}, err
		}
	}

	// Read requests burst
	if burstStr, ok := env["K6_SECRET_SOURCE_URL_REQUESTS_BURST"]; ok && burstStr != "" {
		if err := parseIntOption(burstStr, &cfg.RequestsBurst, "requestsBurst"); err != nil {
			return extConfig{}, err
		}
	}

	// Read headers - iterate through all environment variables
	// Headers are prefixed with K6_SECRET_SOURCE_URL_HEADER_
	for key, value := range env {
		if strings.HasPrefix(key, "K6_SECRET_SOURCE_URL_HEADER_") {
			headerName := strings.TrimPrefix(key, "K6_SECRET_SOURCE_URL_HEADER_")
			if cfg.Headers == nil {
				cfg.Headers = make(map[string]string)
			}
			cfg.Headers[headerName] = value
		}
	}

	return cfg, nil
}

func loadConfigFromFile(configPath string, fs fsext.Fs) (extConfig, error) {
	file, err := fs.Open(configPath)
	if err != nil {
		return extConfig{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer func() { _ = file.Close() }()

	configData, err := io.ReadAll(file)
	if err != nil {
		return extConfig{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var fileCfg extConfig
	if err := json.Unmarshal(configData, &fileCfg); err != nil {
		return extConfig{}, fmt.Errorf("failed to parse JSON config: %w", err)
	}

	return fileCfg, nil
}

func validateURLTemplate(urlTemplate string) error {
	if urlTemplate == "" {
		return errMissingURLTemplate
	}

	// Require {key} placeholder to differentiate between secrets
	if !strings.Contains(urlTemplate, "{key}") {
		return errors.New("urlTemplate must contain {key} placeholder")
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

func getConfig(arg string, fs fsext.Fs, env map[string]string) (extConfig, error) {
	// Start with defaults
	config := newConfig()

	// Apply environment variables
	// Order of precedence (lowest to highest):
	// 1. Defaults
	// 2. Environment variables
	// 3. Config file (if specified)
	// 4. Inline CLI flags
	envCfg, err := parseEnvConfig(env)
	if err != nil {
		return extConfig{}, err
	}
	config = config.Apply(envCfg)

	// If arg is provided and not empty, parse it (file-based or inline config)
	if arg != "" {
		explicitCfg, err := parseInlineConfig(arg, fs)
		if err != nil {
			return extConfig{}, err
		}
		config = config.Apply(explicitCfg)
	}

	// Validate the final config
	if err := validateConfig(config); err != nil {
		return extConfig{}, err
	}

	return config, nil
}

func validateConfig(config extConfig) error {
	// Validate required fields and value constraints
	if err := validateURLTemplate(config.URLTemplate); err != nil {
		return err
	}

	if config.Timeout.Duration <= 0 {
		return errors.New("timeout must be greater than 0")
	}

	if config.RequestsPerMinuteLimit.Int64 <= 0 {
		return errors.New("requestsPerMinuteLimit must be greater than 0")
	}

	if config.RequestsBurst.Int64 <= 0 {
		return errors.New("requestsBurst must be greater than 0")
	}

	if config.MaxRetries.Int64 < 0 {
		return errors.New("maxRetries must be non-negative")
	}

	if config.RetryBackoff.Duration <= 0 {
		return errors.New("retryBackoff must be greater than 0")
	}

	return nil
}
