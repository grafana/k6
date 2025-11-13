package url

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/fsext"
)

const testConfigFile = "/config.json"

func TestParseConfigArgument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		arg         string
		expected    string
		expectedErr error
	}{
		{
			name:        "valid config argument",
			arg:         "config=/path/to/config.json",
			expected:    "/path/to/config.json",
			expectedErr: nil,
		},
		{
			name:        "missing config key",
			arg:         "/path/to/config.json",
			expected:    "",
			expectedErr: errInvalidConfig,
		},
		{
			name:        "wrong key",
			arg:         "file=/path/to/config.json",
			expected:    "",
			expectedErr: errInvalidConfig,
		},
		{
			name:        "empty argument",
			arg:         "",
			expected:    "",
			expectedErr: errInvalidConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseConfigArgument(tt.arg)
			assert.Equal(t, tt.expected, result)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestExtractSecretFromResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         []byte
		responsePath string
		expected     string
		expectError  bool
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := extractSecretFromResponse(tt.body, tt.responsePath)
			if tt.expectError {
				assert.Error(t, err)
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
			Method:       "GET",
			ResponsePath: "data.value",
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = afero.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		result, err := getConfig("config="+testConfigFile, fs)
		require.NoError(t, err)
		assert.Equal(t, config.URLTemplate, result.URLTemplate)
		assert.Equal(t, config.Headers, result.Headers)
		assert.Equal(t, config.Method, result.Method)
		assert.Equal(t, config.ResponsePath, result.ResponsePath)
		assert.Equal(t, defaultRequestsPerMinuteLimit, *result.RequestsPerMinuteLimit)
		assert.Equal(t, defaultRequestsBurst, *result.RequestsBurst)
		assert.Equal(t, defaultTimeoutSeconds, *result.TimeoutSeconds)
	})

	t.Run("custom rate limits", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		customLimit := 100
		customBurst := 5
		customTimeout := 60
		config := extConfig{
			URLTemplate:            "https://api.example.com/secrets/{key}",
			RequestsPerMinuteLimit: &customLimit,
			RequestsBurst:          &customBurst,
			TimeoutSeconds:         &customTimeout,
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = afero.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		result, err := getConfig("config="+testConfigFile, fs)
		require.NoError(t, err)
		assert.Equal(t, customLimit, *result.RequestsPerMinuteLimit)
		assert.Equal(t, customBurst, *result.RequestsBurst)
		assert.Equal(t, customTimeout, *result.TimeoutSeconds)
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
		err = afero.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		_, err = getConfig("config="+testConfigFile, fs)
		assert.ErrorIs(t, err, errMissingURLTemplate)
	})

	t.Run("invalid rate limit", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()

		invalidLimit := -1
		config := extConfig{
			URLTemplate:            "https://api.example.com/secrets/{key}",
			RequestsPerMinuteLimit: &invalidLimit,
		}

		configData, err := json.Marshal(config)
		require.NoError(t, err)
		err = afero.WriteFile(fs, testConfigFile, configData, 0o600)
		require.NoError(t, err)

		_, err = getConfig("config="+testConfigFile, fs)
		assert.ErrorIs(t, err, errInvalidRequestsPerMinuteLimit)
	})

	t.Run("nonexistent config file", func(t *testing.T) {
		t.Parallel()
		fs := fsext.NewMemMapFs()
		_, err := getConfig("config=/nonexistent/file.json", fs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open config file")
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
		us := &urlSecrets{
			config: extConfig{
				URLTemplate: server.URL + "/secrets/{key}",
				Headers: map[string]string{
					"Authorization": "Bearer token123",
				},
				Method:         "GET",
				ResponsePath:   "",
				TimeoutSeconds: &timeout,
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
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
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:    server.URL + "/api/secrets/{key}",
				Method:         "GET",
				ResponsePath:   "data.value",
				TimeoutSeconds: &timeout,
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
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
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:    server.URL + "/secrets/{key}",
				TimeoutSeconds: &timeout,
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
		}

		_, err := us.Get("nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, errFailedToGetSecret)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("rate limiter error", func(t *testing.T) {
		t.Parallel()
		us := &urlSecrets{
			config:     extConfig{},
			httpClient: &http.Client{},
			limiter:    &mockLimiter{shouldError: true},
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
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:    server.URL + "/secrets/{key}",
				TimeoutSeconds: &timeout,
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
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

func TestURLSecrets_Description(t *testing.T) {
	t.Parallel()
	us := &urlSecrets{
		config: extConfig{
			URLTemplate: "https://vault.example.com/api/{key}",
		},
	}

	desc := us.Description()
	assert.Contains(t, desc, "URL-based secret source")
	assert.Contains(t, desc, "https://vault.example.com/api/{key}")
}

// TestURLSecrets_GSM_Integration tests that the URL secret source works as a drop-in
// replacement for the GSM (Grafana Secrets Manager) extension.
func TestURLSecrets_GSM_Integration(t *testing.T) {
	t.Parallel()

	t.Run("GSM decrypt endpoint with plaintext response", func(t *testing.T) {
		t.Parallel()
		// Mock GSM server that returns DecryptedSecret response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Verify the request matches GSM's DecryptSecretById format
			assert.Equal(t, "GET", req.Method)
			assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))

			// Extract secret ID from path
			expectedPath := "/secrets/my-secret-id/decrypt"
			assert.Equal(t, expectedPath, req.URL.Path)

			// Return a response matching GSM's DecryptedSecret format
			response := map[string]interface{}{
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

		// Configure URL secret source to match GSM format
		timeout := 5
		us := &urlSecrets{
			config: extConfig{
				URLTemplate: server.URL + "/secrets/{key}/decrypt",
				Headers: map[string]string{
					"Authorization": "Bearer test-token",
				},
				Method:         "GET",
				ResponsePath:   "plaintext",
				TimeoutSeconds: &timeout,
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
		}

		// Test fetching a secret
		secret, err := us.Get("my-secret-id")
		require.NoError(t, err)
		assert.Equal(t, "super-secret-value", secret)
	})

	t.Run("GSM with multiple secret IDs", func(t *testing.T) {
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

			response := map[string]interface{}{
				"name":      secretID,
				"plaintext": plaintext,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		timeout := 5
		us := &urlSecrets{
			config: extConfig{
				URLTemplate: server.URL + "/secrets/{key}/decrypt",
				Headers: map[string]string{
					"Authorization": "Bearer gsm-token",
				},
				ResponsePath:   "plaintext",
				TimeoutSeconds: &timeout,
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
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

	t.Run("GSM error responses", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// Simulate GSM error (e.g., secret not found)
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
		us := &urlSecrets{
			config: extConfig{
				URLTemplate:    server.URL + "/secrets/{key}/decrypt",
				ResponsePath:   "plaintext",
				TimeoutSeconds: &timeout,
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
		}

		_, err := us.Get("nonexistent-secret")
		assert.Error(t, err)
		assert.ErrorIs(t, err, errFailedToGetSecret)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("GSM complete URL format", func(t *testing.T) {
		t.Parallel()
		// Test with the exact GSM URL format specified by the user
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// The URL should be in format: /secrets/{id}/decrypt
			assert.True(t, strings.HasPrefix(req.URL.Path, "/secrets/"))
			assert.True(t, strings.HasSuffix(req.URL.Path, "/decrypt"))

			response := map[string]interface{}{
				"plaintext": "secret-from-gsm",
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		timeout := 5
		us := &urlSecrets{
			config: extConfig{
				// Format matching: https://gsm.proxy-lb:8080/secrets/%s/decrypt
				URLTemplate:    server.URL + "/secrets/{key}/decrypt",
				ResponsePath:   "plaintext",
				TimeoutSeconds: &timeout,
			},
			httpClient: &http.Client{Timeout: 5 * time.Second},
			limiter:    &mockLimiter{},
		}

		secret, err := us.Get("test-secret-123")
		require.NoError(t, err)
		assert.Equal(t, "secret-from-gsm", secret)
	})
}

func TestNewLimiter(t *testing.T) {
	t.Parallel()
	limiter := newLimiter(60, 10)
	require.NotNil(t, limiter)

	// Test that limiter allows burst
	ctx := context.Background()
	for i := 0; i < 10; i++ {
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
