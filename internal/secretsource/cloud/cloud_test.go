package cloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/lib/fsext"
	"go.k6.io/k6/v2/secretsource"

	_ "go.k6.io/k6/v2/internal/secretsource/url"
)

func getCloudConstructor(t *testing.T) secretsource.Constructor {
	t.Helper()
	extensions := ext.Get(ext.SecretSourceExtension)
	cloudExt, ok := extensions["cloud"]
	require.True(t, ok, "cloud secret source not registered")
	constructor, ok := cloudExt.Module.(secretsource.Constructor)
	require.True(t, ok, "cloud secret source has invalid type")
	return constructor
}

func makeParams() secretsource.Params {
	return secretsource.Params{
		Logger:      logrus.New(),
		Environment: map[string]string{},
		FS:          fsext.NewMemMapFs(),
	}
}

// TestCloudSecretsRequiresConfig verifies construction succeeds but Get fails with no config.
func TestCloudSecretsRequiresConfig(t *testing.T) {
	t.Parallel()

	source, err := New(makeParams())
	require.NoError(t, err, "construction should succeed")

	// Error should occur on Get() when no config is set
	_, err = source.Get("test-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cloud secrets not configured")
}

// TestCloudSecretsDescription verifies the extension is registered with the expected description.
func TestCloudSecretsDescription(t *testing.T) {
	t.Parallel()

	constructor := getCloudConstructor(t)
	source, err := constructor(makeParams())
	require.NoError(t, err)
	require.NotNil(t, source)

	require.Equal(t, "Grafana Cloud k6 secret source", source.Description())
}

// TestCloudSecretsGet verifies a successful secret fetch via an httptest server.
func TestCloudSecretsGet(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"plaintext": "my-secret-value"})
	}))
	defer server.Close()

	source, err := New(makeParams())
	require.NoError(t, err)
	source.SetConfig(&Config{
		Token:        "test-token-123",
		Endpoint:     server.URL + "/{key}",
		ResponsePath: "plaintext",
	})

	value, err := source.Get("MY_KEY")
	require.NoError(t, err)
	require.Equal(t, "my-secret-value", value)
}

// TestCloudSecretsGetAuthHeader verifies that the Authorization header is set correctly.
func TestCloudSecretsGetAuthHeader(t *testing.T) {
	t.Parallel()

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"plaintext": "secret"})
	}))
	defer server.Close()

	source, err := New(makeParams())
	require.NoError(t, err)
	source.SetConfig(&Config{
		Token:        "test-token-123",
		Endpoint:     server.URL + "/{key}",
		ResponsePath: "plaintext",
	})

	_, err = source.Get("MY_KEY")
	require.NoError(t, err)
	assert.Equal(t, "Bearer test-token-123", gotAuth)
}

// TestCloudSecretsGetConcurrent verifies that 50 goroutines calling Get() concurrently is safe.
func TestCloudSecretsGetConcurrent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"plaintext": "concurrent-secret"})
	}))
	defer server.Close()

	source, err := New(makeParams())
	require.NoError(t, err)
	source.SetConfig(&Config{
		Token:        "concurrent-token",
		Endpoint:     server.URL + "/{key}",
		ResponsePath: "plaintext",
	})

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			value, getErr := source.Get("concurrent-key")
			assert.NoError(t, getErr)
			assert.Equal(t, "concurrent-secret", value)
		}()
	}
	wg.Wait()
}

// TestCloudSecretsGetResponsePath verifies nested JSON path extraction.
func TestCloudSecretsGetResponsePath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{"value": "nested-secret"},
		})
	}))
	defer server.Close()

	source, err := New(makeParams())
	require.NoError(t, err)
	source.SetConfig(&Config{
		Token:        "path-token",
		Endpoint:     server.URL + "/{key}",
		ResponsePath: "data.value",
	})

	value, err := source.Get("nested-key")
	require.NoError(t, err)
	require.Equal(t, "nested-secret", value)
}

// TestCloudSecretsGetMissingToken verifies that an empty token produces a clear error.
func TestCloudSecretsGetMissingToken(t *testing.T) {
	t.Parallel()

	source, err := New(makeParams())
	require.NoError(t, err)
	source.SetConfig(&Config{Token: "", Endpoint: "https://example.com/{key}"})

	_, err = source.Get("key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "token not set")
}

// TestCloudSecretsGetMissingEndpoint verifies that an empty endpoint produces a clear error.
func TestCloudSecretsGetMissingEndpoint(t *testing.T) {
	t.Parallel()

	source, err := New(makeParams())
	require.NoError(t, err)
	source.SetConfig(&Config{Token: "some-token", Endpoint: ""})

	_, err = source.Get("key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint not set")
}

// TestCloudSecretsMultipleRuns verifies that a single pre-registered source correctly
// re-initializes when SetConfig is called for a second sequential test run.
// With sync.Once this would silently use the first run's credentials forever.
func TestCloudSecretsMultipleRuns(t *testing.T) {
	t.Parallel()

	makeServer := func(secret string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"plaintext": secret})
		}))
	}

	server1 := makeServer("secret-run-1")
	defer server1.Close()
	server2 := makeServer("secret-run-2")
	defer server2.Close()

	// Pre-registered once at startup (createSecretSources)
	source, err := New(makeParams())
	require.NoError(t, err)

	// First test run
	source.SetConfig(&Config{Token: "token-run-1", Endpoint: server1.URL + "/{key}", ResponsePath: "plaintext"})
	val, err := source.Get("mykey")
	require.NoError(t, err)
	require.Equal(t, "secret-run-1", val)

	// Second test run — SetConfig with different credentials
	source.SetConfig(&Config{Token: "token-run-2", Endpoint: server2.URL + "/{key}", ResponsePath: "plaintext"})
	val, err = source.Get("mykey")
	require.NoError(t, err)
	require.Equal(t, "secret-run-2", val, "second run must use new credentials, not cached ones from first run")
}

// TestSetConfigAndGet verifies the normal flow: SetConfig then Get via a pre-registered source.
// This mirrors how createSecretSources (root.go) pre-registers the source and createCloudTest
// (outputs_cloud.go) calls SetConfig before VUs start.
func TestSetConfigAndGet(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"plaintext": "preconfigured-secret"})
	}))
	defer server.Close()

	// Simulate createSecretSources: pre-register source
	cloudSource, err := New(makeParams())
	require.NoError(t, err)
	manager, _, err := secretsource.NewManager(map[string]secretsource.Source{
		"cloud":   cloudSource,
		"default": cloudSource,
	})
	require.NoError(t, err)

	// Simulate createCloudTest: SetConfig called after manager creation, before VUs start
	cloudSource.SetConfig(&Config{
		Token:        "pre-config-token",
		Endpoint:     server.URL + "/{key}",
		ResponsePath: "plaintext",
	})

	val, err := manager.Get("cloud", "mykey")
	require.NoError(t, err)
	require.Equal(t, "preconfigured-secret", val)

	val, err = manager.Get("default", "mykey")
	require.NoError(t, err)
	require.Equal(t, "preconfigured-secret", val)
}
