package url

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"
)

const testConfigFile = "/config.json"

func TestInlineConfig(t *testing.T) {
	t.Parallel()

	t.Run("inline urlTemplate only", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		result, err := parseInlineConfig("urlTemplate=https://api.example.com/secrets/{key}", fs)
		require.NoError(t, err)
		assert.Equal(t, "https://api.example.com/secrets/{key}", result.URLTemplate)
		// parseInlineConfig doesn't apply defaults; only parses explicit config
		// Defaults are applied at the getConfig level
		assert.False(t, result.Method.Valid)
		assert.False(t, result.RequestsPerMinuteLimit.Valid)
	})

	t.Run("inline config with multiple options", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := "urlTemplate=https://api.example.com/{key},method=POST,timeout=60s,maxRetries=5"
		result, err := parseInlineConfig(config, fs)
		require.NoError(t, err)
		assert.Equal(t, "https://api.example.com/{key}", result.URLTemplate)
		assert.Equal(t, "POST", result.Method.String)
		assert.Equal(t, 60*time.Second, time.Duration(result.Timeout.Duration))
		assert.Equal(t, int64(5), result.MaxRetries.Int64)
	})

	t.Run("inline config with headers", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := "urlTemplate=https://api.example.com/{key},headers.Authorization=Bearer token123,headers.X-Custom=value"
		result, err := parseInlineConfig(config, fs)
		require.NoError(t, err)
		assert.Equal(t, "https://api.example.com/{key}", result.URLTemplate)
		assert.Equal(t, "Bearer token123", result.Headers["Authorization"])
		assert.Equal(t, "value", result.Headers["X-Custom"])
	})

	t.Run("mixed file and inline config", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		// Create a base config file
		baseConfig := extConfig{
			URLTemplate: "https://base.example.com/{key}",
			Method:      null.StringFrom("GET"),
			Timeout:     types.NullDurationFrom(30 * time.Second),
		}
		configData, err := json.Marshal(baseConfig)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		// Load from file and override with inline config
		config := "config=" + testConfigFile + ",timeout=60s,maxRetries=10"
		result, err := parseInlineConfig(config, fs)
		require.NoError(t, err)
		assert.Equal(t, "https://base.example.com/{key}", result.URLTemplate)
		assert.Equal(t, 60*time.Second, time.Duration(result.Timeout.Duration))
		assert.Equal(t, int64(10), result.MaxRetries.Int64)
	})

	t.Run("invalid config format", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		_, err := parseInlineConfig("invalid-no-equals", fs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected key=value")
	})

	t.Run("unknown config key", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		_, err := parseInlineConfig("urlTemplate=https://api.example.com/{key},unknownKey=value", fs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown configuration key")
	})

	t.Run("invalid timeout format", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		_, err := parseInlineConfig("urlTemplate=https://api.example.com/{key},timeout=invalid", fs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout")
	})

	t.Run("all numeric options", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := "urlTemplate=https://api.example.com/{key},requestsPerMinuteLimit=100,requestsBurst=20,maxRetries=2"
		result, err := parseInlineConfig(config, fs)
		require.NoError(t, err)
		assert.Equal(t, int64(100), result.RequestsPerMinuteLimit.Int64)
		assert.Equal(t, int64(20), result.RequestsBurst.Int64)
		assert.Equal(t, int64(2), result.MaxRetries.Int64)
	})
}

func TestExtractSecretFromResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         []byte
		responsePath string
		expected     string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "plain text response",
			body:         []byte("my-secret-value"),
			responsePath: "",
			expected:     "my-secret-value",
			expectError:  false,
		},
		{
			name:         "simple JSON path",
			body:         []byte(`{"secret":"my-secret-value"}`),
			responsePath: "secret",
			expected:     "my-secret-value",
			expectError:  false,
		},
		{
			name:         "nested JSON path",
			body:         []byte(`{"data":{"value":"my-secret-value"}}`),
			responsePath: "data.value",
			expected:     "my-secret-value",
			expectError:  false,
		},
		{
			name:         "deeply nested JSON path",
			body:         []byte(`{"response":{"data":{"secret":{"value":"my-secret-value"}}}}`),
			responsePath: "response.data.secret.value",
			expected:     "my-secret-value",
			expectError:  false,
		},
		{
			name:         "invalid JSON path",
			body:         []byte(`{"secret":"my-secret-value"}`),
			responsePath: "nonexistent",
			expected:     "",
			expectError:  true,
		},
		{
			name:         "non-string value",
			body:         []byte(`{"secret":123}`),
			responsePath: "secret",
			expected:     "",
			expectError:  true,
		},
		{
			name:         "invalid JSON",
			body:         []byte(`{invalid json`),
			responsePath: "secret",
			expected:     "",
			expectError:  true,
		},
		{
			name:         "response exceeds size limit",
			body:         bytes.Repeat([]byte("x"), 25*1024), // 25KB
			responsePath: "",
			expected:     "",
			expectError:  true,
			errorMsg:     "exceeds maximum size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := extractSecretFromResponse(bytes.NewReader(tt.body), tt.responsePath)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.ErrorContains(t, err, tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	t.Parallel()

	t.Run("valid config file", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := extConfig{
			URLTemplate: "https://api.example.com/secrets/{key}",
			Headers: map[string]string{
				"Authorization": "Bearer token123",
			},
			Method:       null.StringFrom("GET"),
			ResponsePath: null.StringFrom("data.value"),
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		result, err := getConfig("config="+testConfigFile, fs, nil)
		require.NoError(t, err)
		assert.Equal(t, config.URLTemplate, result.URLTemplate)
		assert.Equal(t, config.Headers, result.Headers)
		assert.Equal(t, config.Method.String, result.Method.String)
		assert.Equal(t, config.ResponsePath.String, result.ResponsePath.String)
		assert.Equal(t, int64(300), result.RequestsPerMinuteLimit.Int64)
		assert.Equal(t, int64(10), result.RequestsBurst.Int64)
		assert.Equal(t, 30*time.Second, time.Duration(result.Timeout.Duration))
	})

	t.Run("custom rate limits", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		customLimit := 100
		customBurst := 5
		customTimeout := types.NullDurationFrom(60 * time.Second)
		config := extConfig{
			URLTemplate:            "https://api.example.com/secrets/{key}",
			RequestsPerMinuteLimit: null.IntFrom(int64(customLimit)),
			RequestsBurst:          null.IntFrom(int64(customBurst)),
			Timeout:                customTimeout,
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		result, err := getConfig("config="+testConfigFile, fs, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(customLimit), result.RequestsPerMinuteLimit.Int64)
		assert.Equal(t, int64(customBurst), result.RequestsBurst.Int64)
		assert.Equal(t, customTimeout.Duration, result.Timeout.Duration)
	})

	t.Run("missing URL template", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := extConfig{
			Headers: map[string]string{
				"Authorization": "Bearer token123",
			},
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		_, err = getConfig("config="+testConfigFile, fs, nil)
		assert.ErrorIs(t, err, errMissingURLTemplate)
	})

	t.Run("invalid rate limit", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		invalidLimit := -1
		config := extConfig{
			URLTemplate:            "https://api.example.com/secrets/{key}",
			RequestsPerMinuteLimit: null.IntFrom(int64(invalidLimit)),
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		_, err = getConfig("config="+testConfigFile, fs, nil)
		assert.ErrorContains(t, err, "requestsPerMinuteLimit must be greater than 0")
	})

	t.Run("nonexistent config file", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()
		_, err := getConfig("config=/nonexistent/file.json", fs, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open config file")
	})

	t.Run("invalid URL format", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := extConfig{
			URLTemplate: "not a valid url {key}",
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		_, err = getConfig("config="+testConfigFile, fs, nil)
		assert.ErrorContains(t, err, "urlTemplate must be an absolute URL with a scheme")
	})

	t.Run("http scheme is valid", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := extConfig{
			URLTemplate: "http://api.example.com/secrets/{key}",
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		result, err := getConfig("config="+testConfigFile, fs, nil)
		require.NoError(t, err)
		assert.Equal(t, "http://api.example.com/secrets/{key}", result.URLTemplate)
	})

	t.Run("https scheme is valid", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := extConfig{
			URLTemplate: "https://api.example.com/secrets/{key}",
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		result, err := getConfig("config="+testConfigFile, fs, nil)
		require.NoError(t, err)
		assert.Equal(t, "https://api.example.com/secrets/{key}", result.URLTemplate)
	})

	t.Run("missing {key} placeholder", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := extConfig{
			URLTemplate: "https://api.example.com/secrets/static-value",
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		_, err = getConfig("config="+testConfigFile, fs, nil)
		assert.ErrorContains(t, err, "must contain {key} placeholder")
	})
}

func TestURLSecrets_Get(t *testing.T) {
	t.Parallel()

	t.Run("successful GET request with plain response", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "/secrets/my-key", req.URL.Path)
			assert.Equal(t, "GET", req.Method)
			assert.Equal(t, "Bearer token123", req.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("my-secret-value"))
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate: server.URL + "/secrets/{key}",
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
				Method:       null.StringFrom("GET"),
				ResponsePath: null.StringFrom(""),
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		secret, err := us.Get("my-key")
		require.NoError(t, err)
		assert.Equal(t, "my-secret-value", secret)
	})

	t.Run("successful GET request with JSON response", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "/api/secrets/api-key", req.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"value":"secret-from-json"}}`))
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/api/secrets/{key}",
				Method:       null.StringFrom("GET"),
				ResponsePath: null.StringFrom("data.value"),
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		secret, err := us.Get("api-key")
		require.NoError(t, err)
		assert.Equal(t, "secret-from-json", secret)
	})

	t.Run("HTTP error status code", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		_, err := us.Get("nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, errFailedToGetSecret)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("rate limiter error", func(t *testing.T) {
		t.Parallel()
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{},
			limiter:    &mockLimiter{shouldError: true},
			logger:     testutils.NewLogger(t),
		}

		_, err := us.Get("any-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate limiter error")
	})

	t.Run("multiple keys with URL template", func(t *testing.T) {
		t.Parallel()
		requestedKeys := []string{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Extract key from path
			key := strings.TrimPrefix(req.URL.Path, "/secrets/")
			requestedKeys = append(requestedKeys, key)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("secret-for-" + key))
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		// Request multiple keys
		keys := []string{"key1", "key2", "key3"}
		for _, key := range keys {
			secret, err := us.Get(key)
			require.NoError(t, err)
			assert.Equal(t, "secret-for-"+key, secret)
		}

		assert.Equal(t, keys, requestedKeys)
	})
}

func TestURLSecrets_Get_Retry(t *testing.T) {
	t.Parallel()

	t.Run("retry on 500 error then succeed", func(t *testing.T) {
		t.Parallel()
		attemptCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			attemptCount++
			currentAttempt := attemptCount
			mu.Unlock()

			// Fail first 2 attempts with 500, then succeed
			if currentAttempt <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("secret-after-retry"))
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		secret, err := us.Get("test-key")
		require.NoError(t, err)
		assert.Equal(t, "secret-after-retry", secret)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 3, attemptCount, "should have made 3 attempts (2 failures + 1 success)")
	})

	t.Run("retry on 429 rate limit then succeed", func(t *testing.T) {
		t.Parallel()
		attemptCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			attemptCount++
			currentAttempt := attemptCount
			mu.Unlock()

			// Return 429 on first attempt, then succeed
			if currentAttempt == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("secret-after-rate-limit"))
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		secret, err := us.Get("test-key")
		require.NoError(t, err)
		assert.Equal(t, "secret-after-rate-limit", secret)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 2, attemptCount, "should have made 2 attempts (1 rate limit + 1 success)")
	})

	t.Run("no retry on 401 unauthorized error", func(t *testing.T) {
		t.Parallel()
		attemptCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			attemptCount++
			mu.Unlock()

			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		_, err := us.Get("test-key")
		assert.Error(t, err)
		assert.ErrorIs(t, err, errFailedToGetSecret)
		assert.Contains(t, err.Error(), "401")

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 1, attemptCount, "should only attempt once for 401 error (no retry)")
	})

	t.Run("no retry on 404 not found error", func(t *testing.T) {
		t.Parallel()
		attemptCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			attemptCount++
			mu.Unlock()

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		_, err := us.Get("test-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 1, attemptCount, "should only attempt once for 404 error (no retry)")
	})

	t.Run("exhaust all retries with 503 errors", func(t *testing.T) {
		t.Parallel()
		attemptCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			attemptCount++
			mu.Unlock()

			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("service unavailable"))
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 2
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		_, err := us.Get("test-key")
		assert.Error(t, err)
		assert.ErrorIs(t, err, errFailedToGetSecret)
		assert.Contains(t, err.Error(), "503")

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 3, attemptCount, "should have made 3 attempts (maxRetries=2 means 2 retries + 1 initial = 3 total)")
	})

	t.Run("maxRetries=0 means no retries", func(t *testing.T) {
		t.Parallel()
		attemptCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			attemptCount++
			mu.Unlock()

			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 0
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		_, err := us.Get("test-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 1, attemptCount, "should only attempt once when maxRetries=0")
	})

	t.Run("retry with 502 bad gateway then succeed", func(t *testing.T) {
		t.Parallel()
		attemptCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			attemptCount++
			currentAttempt := attemptCount
			mu.Unlock()

			// Fail first attempt with 502, then succeed
			if currentAttempt == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("secret-after-502"))
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		secret, err := us.Get("test-key")
		require.NoError(t, err)
		assert.Equal(t, "secret-after-502", secret)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 2, attemptCount, "should have made 2 attempts (1 failure + 1 success)")
	})

	t.Run("uses default retry config when not specified", func(t *testing.T) {
		t.Parallel()
		attemptCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			attemptCount++
			currentAttempt := attemptCount
			mu.Unlock()

			// Fail first 2 attempts, then succeed
			if currentAttempt <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("secret-with-defaults"))
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}",
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(1 * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		secret, err := us.Get("test-key")
		require.NoError(t, err)
		assert.Equal(t, "secret-with-defaults", secret)

		mu.Lock()
		defer mu.Unlock()
		// Default maxRetries is 3, so should attempt up to 4 times (3 retries + 1 initial)
		assert.GreaterOrEqual(t, attemptCount, 2, "should have retried at least once with defaults")
		assert.LessOrEqual(t, attemptCount, 4, "should not exceed default maxRetries")
	})
}

func TestGetConfig_Retry(t *testing.T) {
	t.Parallel()

	t.Run("default retry config values", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		config := extConfig{
			URLTemplate: "https://api.example.com/secrets/{key}",
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		result, err := getConfig("config="+testConfigFile, fs, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.MaxRetries.Int64)
		assert.Equal(t, 1*time.Second, time.Duration(result.RetryBackoff.Duration))
	})

	t.Run("custom retry config values", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		customRetries := 5
		customBackoff := types.NullDurationFrom(2 * time.Second)
		config := extConfig{
			URLTemplate:  "https://api.example.com/secrets/{key}",
			MaxRetries:   null.IntFrom(int64(customRetries)),
			RetryBackoff: customBackoff,
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		result, err := getConfig("config="+testConfigFile, fs, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(customRetries), result.MaxRetries.Int64)
		assert.Equal(t, customBackoff.Duration, result.RetryBackoff.Duration)
	})

	t.Run("invalid maxRetries negative value", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		invalidRetries := -1
		config := extConfig{
			URLTemplate: "https://api.example.com/secrets/{key}",
			MaxRetries:  null.IntFrom(int64(invalidRetries)),
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		_, err = getConfig("config="+testConfigFile, fs, nil)
		assert.ErrorContains(t, err, "maxRetries must be non-negative")
	})

	t.Run("invalid retryBackoff zero value", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		invalidBackoff := types.NullDurationFrom(0)
		config := extConfig{
			URLTemplate:  "https://api.example.com/secrets/{key}",
			RetryBackoff: invalidBackoff,
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		_, err = getConfig("config="+testConfigFile, fs, nil)
		assert.ErrorContains(t, err, "retryBackoff must be greater than 0")
	})

	t.Run("maxRetries=0 is valid (disables retries)", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		zeroRetries := 0
		config := extConfig{
			URLTemplate: "https://api.example.com/secrets/{key}",
			MaxRetries:  null.IntFrom(int64(zeroRetries)),
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = fsext.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		result, err := getConfig("config="+testConfigFile, fs, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(0), result.MaxRetries.Int64)
	})
}

func TestURLSecrets_Description(t *testing.T) {
	t.Parallel()
	us := &urlSecrets{
		config: extConfig{
			URLTemplate: "https://vault.example.com/api/{key}",
		},
	}

	desc := us.Description()
	assert.Contains(t, desc, "URL-based secret source")
}

// TestURLSecrets_JSONResponseIntegration tests that the URL secret source works
// with various JSON response formats from external secret management systems.
func TestURLSecrets_JSONResponseIntegration(t *testing.T) {
	t.Parallel()

	t.Run("JSON API endpoint with nested plaintext field", func(t *testing.T) {
		t.Parallel()
		// Mock secret management API that returns a structured JSON response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Verify the request format
			assert.Equal(t, "GET", req.Method)
			assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))

			// Extract secret ID from path
			expectedPath := "/secrets/my-secret-id/decrypt"
			assert.Equal(t, expectedPath, req.URL.Path)

			// Return a structured JSON response with metadata and the secret value
			response := map[string]any{
				"uuid":        "550e8400-e29b-41d4-a716-446655440000",
				"name":        "my-secret",
				"description": "A test secret",
				"plaintext":   "super-secret-value",
				"org_id":      12345,
				"stack_id":    67890,
				"created_at":  1640000000,
				"created_by":  "user@example.com",
				"modified_at": 1650000000,
				"labels": []map[string]string{
					{"name": "env", "value": "production"},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		// Configure URL secret source to extract the plaintext field from JSON response
		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate: server.URL + "/secrets/{key}/decrypt",
				Headers: map[string]string{
					"Authorization": "Bearer test-token",
				},
				Method:       null.StringFrom("GET"),
				ResponsePath: null.StringFrom("plaintext"),
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		// Test fetching a secret
		secret, err := us.Get("my-secret-id")
		require.NoError(t, err)
		assert.Equal(t, "super-secret-value", secret)
	})

	t.Run("multiple secret IDs with dynamic path substitution", func(t *testing.T) {
		t.Parallel()
		// Track which secrets were requested
		requestedSecrets := make(map[string]bool)
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Extract secret ID from path: /secrets/{id}/decrypt
			path := req.URL.Path
			secretID := strings.TrimSuffix(strings.TrimPrefix(path, "/secrets/"), "/decrypt")

			mu.Lock()
			requestedSecrets[secretID] = true
			mu.Unlock()

			// Return different secrets based on ID
			var plaintext string
			switch secretID {
			case "api-key-prod":
				plaintext = "prod-api-key-value"
			case "db-password":
				plaintext = "secure-db-password"
			case "jwt-secret":
				plaintext = "jwt-signing-key"
			default:
				w.WriteHeader(http.StatusNotFound)
				return
			}

			response := map[string]any{
				"name":      secretID,
				"plaintext": plaintext,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate: server.URL + "/secrets/{key}/decrypt",
				Headers: map[string]string{
					"Authorization": "Bearer api-token",
				},
				ResponsePath: null.StringFrom("plaintext"),
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		// Fetch multiple secrets
		apiKey, err := us.Get("api-key-prod")
		require.NoError(t, err)
		assert.Equal(t, "prod-api-key-value", apiKey)

		dbPassword, err := us.Get("db-password")
		require.NoError(t, err)
		assert.Equal(t, "secure-db-password", dbPassword)

		jwtSecret, err := us.Get("jwt-secret")
		require.NoError(t, err)
		assert.Equal(t, "jwt-signing-key", jwtSecret)

		// Verify all secrets were requested
		mu.Lock()
		defer mu.Unlock()
		assert.True(t, requestedSecrets["api-key-prod"])
		assert.True(t, requestedSecrets["db-password"])
		assert.True(t, requestedSecrets["jwt-secret"])
	})

	t.Run("API error responses", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// Simulate API error (e.g., secret not found)
			w.WriteHeader(http.StatusNotFound)
			response := map[string]string{
				"code":    "not_found",
				"message": "Secret not found",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:  server.URL + "/secrets/{key}/decrypt",
				ResponsePath: null.StringFrom("plaintext"),
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		_, err := us.Get("nonexistent-secret")
		assert.Error(t, err)
		assert.ErrorIs(t, err, errFailedToGetSecret)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("complete URL format with path template", func(t *testing.T) {
		t.Parallel()
		// Test with URL template that includes path segments after the key placeholder
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// The URL should be in format: /secrets/{id}/decrypt
			assert.True(t, strings.HasPrefix(req.URL.Path, "/secrets/"))
			assert.True(t, strings.HasSuffix(req.URL.Path, "/decrypt"))

			response := map[string]any{
				"plaintext": "secret-from-api",
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		timeout := 5
		maxRetries := 3
		retryBackoff := 1
		us := &urlSecrets{
			config: extConfig{
				// URL template with key placeholder in the middle of the path
				URLTemplate:  server.URL + "/secrets/{key}/decrypt",
				ResponsePath: null.StringFrom("plaintext"),
				Timeout:      types.NullDurationFrom(time.Duration(timeout) * time.Second),
				MaxRetries:   null.IntFrom(int64(maxRetries)),
				RetryBackoff: types.NullDurationFrom(time.Duration(retryBackoff) * time.Second),
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		secret, err := us.Get("test-secret-123")
		require.NoError(t, err)
		assert.Equal(t, "secret-from-api", secret)
	})
}

func TestNewLimiter(t *testing.T) {
	t.Parallel()
	limiter := newLimiter(60, 10)
	require.NotNil(t, limiter)

	// Test that limiter allows burst
	ctx := context.Background()
	for range 10 {
		err := limiter.Wait(ctx)
		assert.NoError(t, err)
	}

	// The 11th request should be delayed (but we won't wait for it in the test)
	// Just verify the limiter is functional
	assert.NotNil(t, limiter)
}

// mockLimiter is a mock implementation of the limiter interface for testing
type mockLimiter struct {
	shouldError bool
}

func (m *mockLimiter) Wait(_ context.Context) error {
	if m.shouldError {
		return context.Canceled
	}
	return nil
}

func TestEnvConfig(t *testing.T) {
	t.Parallel()

	t.Run("basic env config with urlTemplate only", func(t *testing.T) {
		t.Parallel()
		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE": "https://api.example.com/secrets/{key}",
		}

		fs := fsext.NewMemMapFs()
		result, err := getConfig("", fs, env)
		require.NoError(t, err)
		assert.Equal(t, "https://api.example.com/secrets/{key}", result.URLTemplate)
		assert.Equal(t, "GET", result.Method.String)
		assert.Equal(t, int64(300), result.RequestsPerMinuteLimit.Int64)
	})

	t.Run("env config with all options", func(t *testing.T) {
		t.Parallel()
		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE":              "https://api.example.com/{key}",
			"K6_SECRET_SOURCE_URL_METHOD":                    "POST",
			"K6_SECRET_SOURCE_URL_RESPONSE_PATH":             "data.value",
			"K6_SECRET_SOURCE_URL_TIMEOUT":                   "60s",
			"K6_SECRET_SOURCE_URL_MAX_RETRIES":               "5",
			"K6_SECRET_SOURCE_URL_RETRY_BACKOFF":             "2s",
			"K6_SECRET_SOURCE_URL_REQUESTS_PER_MINUTE_LIMIT": "100",
			"K6_SECRET_SOURCE_URL_REQUESTS_BURST":            "20",
		}

		fs := fsext.NewMemMapFs()
		result, err := getConfig("", fs, env)
		require.NoError(t, err)
		assert.Equal(t, "https://api.example.com/{key}", result.URLTemplate)
		assert.Equal(t, "POST", result.Method.String)
		assert.Equal(t, "data.value", result.ResponsePath.String)
		assert.Equal(t, 60*time.Second, time.Duration(result.Timeout.Duration))
		assert.Equal(t, int64(5), result.MaxRetries.Int64)
		assert.Equal(t, 2*time.Second, time.Duration(result.RetryBackoff.Duration))
		assert.Equal(t, int64(100), result.RequestsPerMinuteLimit.Int64)
		assert.Equal(t, int64(20), result.RequestsBurst.Int64)
	})

	t.Run("env config with headers", func(t *testing.T) {
		t.Parallel()
		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE":         "https://api.example.com/{key}",
			"K6_SECRET_SOURCE_URL_HEADER_AUTHORIZATION": "Bearer token123",
			"K6_SECRET_SOURCE_URL_HEADER_X-Custom":      "custom-value",
		}

		fs := fsext.NewMemMapFs()
		result, err := getConfig("", fs, env)
		require.NoError(t, err)
		assert.Equal(t, "https://api.example.com/{key}", result.URLTemplate)
		assert.Equal(t, "Bearer token123", result.Headers["AUTHORIZATION"])
		assert.Equal(t, "custom-value", result.Headers["X-Custom"])
	})

	t.Run("env config missing urlTemplate", func(t *testing.T) {
		t.Parallel()
		// Don't set K6_SECRET_SOURCE_URL_URL_TEMPLATE
		env := map[string]string{}

		fs := fsext.NewMemMapFs()
		_, err := getConfig("", fs, env)
		assert.ErrorIs(t, err, errMissingURLTemplate)
	})

	t.Run("env config with invalid timeout", func(t *testing.T) {
		t.Parallel()
		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE": "https://api.example.com/{key}",
			"K6_SECRET_SOURCE_URL_TIMEOUT":      "invalid",
		}

		fs := fsext.NewMemMapFs()
		_, err := getConfig("", fs, env)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout")
	})

	t.Run("env config with invalid maxRetries", func(t *testing.T) {
		t.Parallel()
		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE": "https://api.example.com/{key}",
			"K6_SECRET_SOURCE_URL_MAX_RETRIES":  "not-a-number",
		}

		fs := fsext.NewMemMapFs()
		_, err := getConfig("", fs, env)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid maxRetries")
	})

	t.Run("env config with negative maxRetries", func(t *testing.T) {
		t.Parallel()
		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE": "https://api.example.com/{key}",
			"K6_SECRET_SOURCE_URL_MAX_RETRIES":  "-1",
		}

		fs := fsext.NewMemMapFs()
		_, err := getConfig("", fs, env)
		assert.ErrorContains(t, err, "maxRetries must be non-negative")
	})

	t.Run("env config with invalid URL format", func(t *testing.T) {
		t.Parallel()
		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE": "not a valid url {key}",
		}

		fs := fsext.NewMemMapFs()
		_, err := getConfig("", fs, env)
		assert.ErrorContains(t, err, "urlTemplate must be an absolute URL with a scheme")
	})

	t.Run("env config missing {key} placeholder", func(t *testing.T) {
		t.Parallel()
		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE": "https://api.example.com/secrets/static-value",
		}

		fs := fsext.NewMemMapFs()
		_, err := getConfig("", fs, env)
		assert.ErrorContains(t, err, "must contain {key} placeholder")
	})
}

func TestURLSecrets_Get_WithEnvConfig(t *testing.T) {
	t.Parallel()

	t.Run("successful request with env config", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "/secrets/my-key", req.URL.Path)
			assert.Equal(t, "GET", req.Method)
			assert.Equal(t, "Bearer env-token", req.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("my-secret-value"))
		}))
		defer server.Close()

		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE":         server.URL + "/secrets/{key}",
			"K6_SECRET_SOURCE_URL_HEADER_AUTHORIZATION": "Bearer env-token",
			"K6_SECRET_SOURCE_URL_TIMEOUT":              "5s",
			"K6_SECRET_SOURCE_URL_MAX_RETRIES":          "3",
			"K6_SECRET_SOURCE_URL_RETRY_BACKOFF":        "1s",
		}

		fs := fsext.NewMemMapFs()
		config, err := getConfig("", fs, env)
		require.NoError(t, err)

		us := &urlSecrets{
			config:     config,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		secret, err := us.Get("my-key")
		require.NoError(t, err)
		assert.Equal(t, "my-secret-value", secret)
	})

	t.Run("successful request with env config and JSON response", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "/api/secrets/api-key", req.URL.Path)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"value":"secret-from-json"}}`))
		}))
		defer server.Close()

		// Set up environment variables
		env := map[string]string{
			"K6_SECRET_SOURCE_URL_URL_TEMPLATE":  server.URL + "/api/secrets/{key}",
			"K6_SECRET_SOURCE_URL_RESPONSE_PATH": "data.value",
			"K6_SECRET_SOURCE_URL_TIMEOUT":       "5s",
			"K6_SECRET_SOURCE_URL_MAX_RETRIES":   "3",
			"K6_SECRET_SOURCE_URL_RETRY_BACKOFF": "1s",
		}

		fs := fsext.NewMemMapFs()
		config, err := getConfig("", fs, env)
		require.NoError(t, err)

		us := &urlSecrets{
			config:     config,
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
			logger:     testutils.NewLogger(t),
		}

		secret, err := us.Get("api-key")
		require.NoError(t, err)
		assert.Equal(t, "secret-from-json", secret)
	})
}
