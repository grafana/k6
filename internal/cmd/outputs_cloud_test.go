package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/internal/cloudapi/v6/v6test"
	"go.k6.io/k6/v2/internal/cmd/tests"
	"go.k6.io/k6/v2/internal/loader"
	cloudsecrets "go.k6.io/k6/v2/internal/secretsource/cloud"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
	"go.k6.io/k6/v2/lib/types"
	"go.k6.io/k6/v2/metrics"
	"go.k6.io/k6/v2/secretsource"
)

// minimalLoadedAndConfiguredTest builds a loadedAndConfiguredTest with enough
// structure for createCloudTest to reach its early validation checks, without
// requiring a full script load or runner initialisation.
func minimalLoadedAndConfiguredTest(t *testing.T) *loadedAndConfiguredTest {
	t.Helper()

	registry := metrics.NewRegistry()
	allTags := metrics.SystemTagSet(metrics.TagName |
		metrics.TagMethod |
		metrics.TagStatus |
		metrics.TagError |
		metrics.TagCheck |
		metrics.TagGroup)

	u, err := url.Parse("file:///test/test.js")
	require.NoError(t, err)

	preInitState := &lib.TestPreInitState{
		Logger:   tests.NewGlobalTestState(t).Logger,
		Registry: registry,
		RuntimeOptions: lib.RuntimeOptions{
			Env: make(map[string]string),
		},
	}

	lt := &loadedTest{
		source: &loader.SourceData{
			URL: u,
		},
		preInitState: preInitState,
	}

	cfg := Config{}
	cfg.SystemTags = &allTags

	return &loadedAndConfiguredTest{
		loadedTest:    lt,
		derivedConfig: cfg,
	}
}

// minimalLoadedAndConfiguredTestWithScenarios returns a minimal
// loadedAndConfiguredTest with a 1-second constant-VUs scenario, which is
// sufficient for createCloudTest to pass the duration validation check.
// NoArchiveUpload is set to true so that makeArchive (which requires a real
// runner) is never called.
func minimalLoadedAndConfiguredTestWithScenarios(t *testing.T) *loadedAndConfiguredTest {
	t.Helper()

	test := minimalLoadedAndConfiguredTest(t)

	scenario := executor.NewConstantVUsConfig(lib.DefaultScenarioName)
	scenario.VUs = null.NewInt(1, true)
	scenario.Duration = types.NullDurationFrom(1 * time.Second)

	test.derivedConfig.Scenarios = lib.ScenarioConfigs{
		lib.DefaultScenarioName: scenario,
	}
	test.derivedConfig.NoArchiveUpload = null.NewBool(true, true)

	return test
}

// TestCreateCloudTest_StackIDValidation verifies that createCloudTest returns a
// hard error (AC-X03) when no stack ID is present in the configuration — no
// network call should be made.
func TestCreateCloudTest_StackIDValidation(t *testing.T) {
	t.Parallel()

	t.Run("no stack ID returns K6_CLOUD_STACK_ID error", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		ts.Env["K6_CLOUD_TOKEN"] = "test-token"
		// K6_CLOUD_STACK_ID is intentionally absent

		test := minimalLoadedAndConfiguredTest(t)

		err := createCloudTest(ts.GlobalState, test)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "K6_CLOUD_STACK_ID")
	})

	t.Run("no token returns auth error not stack ID error", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		// Neither token nor stack ID set

		test := minimalLoadedAndConfiguredTest(t)

		err := createCloudTest(ts.GlobalState, test)

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "K6_CLOUD_STACK_ID", "auth error should precede stack ID check")
	})
}

// TestCreateCloudTest_SetsTestRunIDEnvVar verifies that after a successful
// provisioning call, the test run ID is stored as a decimal integer string in
// the K6_CLOUDRUN_TEST_RUN_ID env var (AC-108).
func TestCreateCloudTest_SetsTestRunIDEnvVar(t *testing.T) {
	t.Parallel()

	srv := v6test.NewServer(t, v6test.Config{})

	ts := tests.NewGlobalTestState(t)
	ts.Env["K6_CLOUD_TOKEN"] = "test-token"
	ts.Env["K6_CLOUD_STACK_ID"] = "1"
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

	test := minimalLoadedAndConfiguredTestWithScenarios(t)

	err := createCloudTest(ts.GlobalState, test)
	require.NoError(t, err)

	gotID, ok := test.preInitState.RuntimeOptions.Env[testRunIDKey]
	require.True(t, ok, "K6_CLOUDRUN_TEST_RUN_ID env var should be set")
	assert.Equal(t, "123", gotID, "test run ID should match the mock server's default")
}

// TestCreateCloudTest_CloudSecretSourceConfigured verifies that when
// CloudSecretSource is set (--secret-source=cloud), createCloudTest calls the
// v1 /v1/tests endpoint to fetch SecretsConfig and wires it into the source
// (MUST-FIX 1 option 2 — PRD Out-of-Scope #3). The v6 provisioning path
// remains the primary path; the v1 call is a temporary bridge.
func TestCreateCloudTest_CloudSecretSourceConfigured(t *testing.T) {
	t.Parallel()

	var v1TestCalls atomic.Int32

	// v1 mock server: handles POST /v1/tests and returns a SecretsConfig.
	v1Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/tests" {
			v1TestCalls.Add(1)
			resp := map[string]any{
				"reference_id": "v1-discarded",
				"secrets_config": map[string]any{
					"endpoint":      "https://secrets.k6.io/v1/{key}",
					"response_path": "plaintext",
				},
				"test_run_token": "v1-token-ignored",
			}
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(resp))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer v1Srv.Close()

	// v6 provisioning mock server.
	v6Srv := v6test.NewServer(t, v6test.Config{})

	ts := tests.NewGlobalTestState(t)
	ts.Env["K6_CLOUD_TOKEN"] = "test-token"
	ts.Env["K6_CLOUD_STACK_ID"] = "1"
	ts.Env["K6_CLOUD_HOST_V6"] = v6Srv.URL
	ts.Env["K6_CLOUD_HOST"] = v1Srv.URL

	// Create and set CloudSecretSource to simulate --secret-source=cloud.
	cs, err := cloudsecrets.New(secretsource.Params{
		Logger:      ts.Logger,
		Environment: map[string]string{},
	})
	require.NoError(t, err)
	ts.GlobalState.CloudSecretSource = cs

	test := minimalLoadedAndConfiguredTestWithScenarios(t)
	err = createCloudTest(ts.GlobalState, test)
	require.NoError(t, err)

	assert.Equal(t, int32(1), v1TestCalls.Load(),
		"v1 POST /v1/tests must be called once to fetch SecretsConfig when CloudSecretSource is set")
}

// TestCreateCloudTest_CloudSecretSourceNotCalledWhenNil verifies that when
// CloudSecretSource is nil (--secret-source=cloud not set), the v1 endpoint
// is NOT called.
func TestCreateCloudTest_CloudSecretSourceNotCalledWhenNil(t *testing.T) {
	t.Parallel()

	var v1TestCalls atomic.Int32

	v1Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/tests" {
			v1TestCalls.Add(1)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer v1Srv.Close()

	v6Srv := v6test.NewServer(t, v6test.Config{})

	ts := tests.NewGlobalTestState(t)
	ts.Env["K6_CLOUD_TOKEN"] = "test-token"
	ts.Env["K6_CLOUD_STACK_ID"] = "1"
	ts.Env["K6_CLOUD_HOST_V6"] = v6Srv.URL
	ts.Env["K6_CLOUD_HOST"] = v1Srv.URL
	// CloudSecretSource is intentionally left nil

	test := minimalLoadedAndConfiguredTestWithScenarios(t)
	err := createCloudTest(ts.GlobalState, test)
	require.NoError(t, err)

	assert.Equal(t, int32(0), v1TestCalls.Load(),
		"v1 POST /v1/tests must NOT be called when CloudSecretSource is nil")
}

// TestCreateCloudTest_PropagatesMetricsConfig verifies that MetricsPushURL and
// TestRunToken from RuntimeConfig are propagated into the cloud output config.
func TestCreateCloudTest_PropagatesMetricsConfig(t *testing.T) {
	t.Parallel()

	srv := v6test.NewServer(t, v6test.Config{})

	ts := tests.NewGlobalTestState(t)
	ts.Env["K6_CLOUD_TOKEN"] = "test-token"
	ts.Env["K6_CLOUD_STACK_ID"] = "1"
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

	test := minimalLoadedAndConfiguredTestWithScenarios(t)

	err := createCloudTest(ts.GlobalState, test)
	require.NoError(t, err)

	raw, ok := test.derivedConfig.Collectors[builtinOutputCloud.String()]
	require.True(t, ok, "cloud collector config should be set")

	var conf cloudapi.Config
	require.NoError(t, json.Unmarshal(raw, &conf))

	assert.NotEmpty(t, conf.MetricsPushURL.String, "MetricsPushURL should be propagated from RuntimeConfig")
	assert.Equal(t, "mock-test-run-token", conf.TestRunToken.String,
		"TestRunToken should be propagated from RuntimeConfig")
}
