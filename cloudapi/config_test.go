package cloudapi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib/types"
)

func TestConfigApply(t *testing.T) {
	t.Parallel()
	empty := Config{}
	defaults := NewConfig()

	assert.Equal(t, empty, empty.Apply(empty))
	assert.Equal(t, empty, empty.Apply(defaults))
	assert.Equal(t, defaults, defaults.Apply(defaults))
	assert.Equal(t, defaults, defaults.Apply(empty))
	assert.Equal(t, defaults, defaults.Apply(empty).Apply(empty))

	full := Config{
		Token:                 null.NewString("Token", true),
		StackID:               null.NewInt(1, true),
		ProjectID:             null.NewInt(1, true),
		Name:                  null.NewString("Name", true),
		Host:                  null.NewString("Host", true),
		Hostv6:                null.NewString("Hostv6", true),
		Timeout:               types.NewNullDuration(5*time.Second, true),
		LogsTailURL:           null.NewString("LogsTailURL", true),
		MetricsPushURL:        null.NewString("MetricsPushURL", true),
		TestRunToken:          null.NewString("TestRunToken", true),
		PushRefID:             null.NewString("PushRefID", true),
		WebAppURL:             null.NewString("foo", true),
		NoCompress:            null.NewBool(true, true),
		StopOnError:           null.NewBool(true, true),
		APIVersion:            null.NewInt(2, true),
		AggregationPeriod:     types.NewNullDuration(2*time.Second, true),
		AggregationWaitPeriod: types.NewNullDuration(4*time.Second, true),
		MaxTimeSeriesInBatch:  null.NewInt(3, true),
		MetricPushInterval:    types.NewNullDuration(1*time.Second, true),
		MetricPushConcurrency: null.NewInt(3, true),
		TracesEnabled:         null.NewBool(true, true),
		TracesHost:            null.NewString("TracesHost", true),
		TracesPushInterval:    types.NewNullDuration(10*time.Second, true),
		TracesPushConcurrency: null.NewInt(6, true),
	}

	assert.Equal(t, full, full.Apply(empty))
	assert.Equal(t, full, full.Apply(defaults))
	assert.Equal(t, full, full.Apply(full))
	assert.Equal(t, full, empty.Apply(full))
	assert.Equal(t, full, defaults.Apply(full))
}

// The scoped test_run_token is short-lived and run-scoped; it is now
// deliberately accepted via env (K6_CLOUD_TEST_RUN_TOKEN) so an
// external orchestrator that provisioned the run can hand it to a
// local k6. It still never touches lib.Options / the archive — the
// long-lived org token remains the sensitive credential and is
// unaffected here.
func TestConfig_NewFieldsRoundTripThroughJSON(t *testing.T) {
	t.Parallel()

	original := Config{
		MetricsPushURL: null.StringFrom("https://ingest.example/metrics/abc"),
		TestRunToken:   null.StringFrom("scoped-token-xyz"),
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"metricsPushURL":"https://ingest.example/metrics/abc"`)
	assert.Contains(t, string(data), `"testRunToken":"scoped-token-xyz"`)

	var roundTripped Config
	require.NoError(t, json.Unmarshal(data, &roundTripped))

	assert.Equal(t, original.MetricsPushURL, roundTripped.MetricsPushURL)
	assert.Equal(t, original.TestRunToken, roundTripped.TestRunToken)
}

// TestConfig_NewFieldsFromEnvconfig verifies the scoped push creds are
// read from the environment via envconfig (K6_CLOUD_METRICS_PUSH_URL /
// K6_CLOUD_TEST_RUN_TOKEN) so an external orchestrator that provisioned
// a run can hand them to a local k6.
func TestConfig_NewFieldsFromEnvconfig(t *testing.T) {
	t.Parallel()

	envVars := map[string]string{
		"K6_CLOUD_METRICS_PUSH_URL": "https://push.example/m",
		"K6_CLOUD_TEST_RUN_TOKEN":   "scoped-xyz",
	}

	config, _, err := GetConsolidatedConfig(nil, envVars, "", nil)
	require.NoError(t, err)

	assert.Equal(t, null.StringFrom("https://push.example/m"), config.MetricsPushURL)
	assert.Equal(t, null.StringFrom("scoped-xyz"), config.TestRunToken)
}

func TestConfig_Apply_MergesNewFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		base         Config
		applied      Config
		wantPushURL  null.String
		wantRunToken null.String
	}{
		{
			name:         "applied populated, base unset → uses applied",
			base:         Config{},
			applied:      Config{MetricsPushURL: null.StringFrom("https://push.example"), TestRunToken: null.StringFrom("tok-1")},
			wantPushURL:  null.StringFrom("https://push.example"),
			wantRunToken: null.StringFrom("tok-1"),
		},
		{
			name:         "applied unset, base populated → keeps base",
			base:         Config{MetricsPushURL: null.StringFrom("https://base.example"), TestRunToken: null.StringFrom("tok-base")},
			applied:      Config{},
			wantPushURL:  null.StringFrom("https://base.example"),
			wantRunToken: null.StringFrom("tok-base"),
		},
		{
			name:         "both populated → applied wins",
			base:         Config{MetricsPushURL: null.StringFrom("https://base.example"), TestRunToken: null.StringFrom("tok-base")},
			applied:      Config{MetricsPushURL: null.StringFrom("https://applied.example"), TestRunToken: null.StringFrom("tok-applied")},
			wantPushURL:  null.StringFrom("https://applied.example"),
			wantRunToken: null.StringFrom("tok-applied"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.base.Apply(tc.applied)
			assert.Equal(t, tc.wantPushURL, got.MetricsPushURL)
			assert.Equal(t, tc.wantRunToken, got.TestRunToken)
		})
	}
}

// TestConfig_LogsFieldsFromEnvconfig verifies the log-push config
// fields are read from the environment via envconfig (K6_CLOUD_LOGS_*)
// so an external orchestrator that provisioned a run can hand the log
// push settings to a local k6.
func TestConfig_LogsFieldsFromEnvconfig(t *testing.T) {
	t.Parallel()

	envVars := map[string]string{
		"K6_CLOUD_LOGS_PUSH_URL":         "https://api.k6.io/logs/v1/test_runs/42",
		"K6_CLOUD_LOGS_LEVEL":            "info",
		"K6_CLOUD_LOGS_LIMIT":            "900",
		"K6_CLOUD_LOGS_PUSH_PERIOD":      "3s",
		"K6_CLOUD_LOGS_MESSAGE_MAX_SIZE": "10000",
		"K6_CLOUD_LOGS_ALLOWED_LABELS":   "lz,level,test_run_id",
	}

	config, _, err := GetConsolidatedConfig(nil, envVars, "", nil)
	require.NoError(t, err)

	assert.Equal(t, null.StringFrom("https://api.k6.io/logs/v1/test_runs/42"), config.LogsPushURL)
	assert.Equal(t, null.StringFrom("info"), config.LogsLevel)
	assert.Equal(t, null.IntFrom(900), config.LogsLimit)
	assert.Equal(t, types.NewNullDuration(3*time.Second, true), config.LogsPushPeriod)
	assert.Equal(t, null.IntFrom(10000), config.LogsMessageMaxSize)
	assert.Equal(t, []string{"lz", "level", "test_run_id"}, config.LogsAllowedLabels)
}

func TestConfig_Apply_MergesLogsFields(t *testing.T) {
	t.Parallel()

	baseCfg := Config{
		LogsPushURL:        null.StringFrom("https://base.example/logs"),
		LogsLevel:          null.StringFrom("warn"),
		LogsLimit:          null.IntFrom(100),
		LogsPushPeriod:     types.NewNullDuration(1*time.Second, true),
		LogsMessageMaxSize: null.IntFrom(1024),
		LogsAllowedLabels:  []string{"level"},
	}
	appliedCfg := Config{
		LogsPushURL:        null.StringFrom("https://applied.example/logs"),
		LogsLevel:          null.StringFrom("info"),
		LogsLimit:          null.IntFrom(900),
		LogsPushPeriod:     types.NewNullDuration(3*time.Second, true),
		LogsMessageMaxSize: null.IntFrom(10000),
		LogsAllowedLabels:  []string{"lz", "test_run_id"},
	}

	cases := []struct {
		name    string
		base    Config
		applied Config
		want    Config
	}{
		{
			name:    "applied populated, base unset → uses applied",
			base:    Config{},
			applied: appliedCfg,
			want:    appliedCfg,
		},
		{
			name:    "applied unset, base populated → keeps base",
			base:    baseCfg,
			applied: Config{},
			want:    baseCfg,
		},
		{
			name:    "both populated → applied wins",
			base:    baseCfg,
			applied: appliedCfg,
			want:    appliedCfg,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.base.Apply(tc.applied)
			assert.Equal(t, tc.want.LogsPushURL, got.LogsPushURL)
			assert.Equal(t, tc.want.LogsLevel, got.LogsLevel)
			assert.Equal(t, tc.want.LogsLimit, got.LogsLimit)
			assert.Equal(t, tc.want.LogsPushPeriod, got.LogsPushPeriod)
			assert.Equal(t, tc.want.LogsMessageMaxSize, got.LogsMessageMaxSize)
			assert.Equal(t, tc.want.LogsAllowedLabels, got.LogsAllowedLabels)
		})
	}
}

func TestGetConsolidatedConfig(t *testing.T) {
	t.Parallel()
	config, warn, err := GetConsolidatedConfig(json.RawMessage(`{"token":"jsonraw"}`), nil, "", nil)
	require.NoError(t, err)
	require.Equal(t, "jsonraw", config.Token.String)
	require.Empty(t, warn)

	config, warn, err = GetConsolidatedConfig(
		json.RawMessage(`{"token":"jsonraw"}`),
		nil,
		"",
		json.RawMessage(`{"token":"ext"}`),
	)
	require.NoError(t, err)
	require.Equal(t, "ext", config.Token.String)
	require.Empty(t, warn)

	config, warn, err = GetConsolidatedConfig(
		json.RawMessage(`{"token":"jsonraw"}`),
		map[string]string{"K6_CLOUD_TOKEN": "envvalue"},
		"",
		json.RawMessage(`{"token":"ext"}`),
	)
	require.NoError(t, err)
	require.Equal(t, "envvalue", config.Token.String)
	require.Empty(t, warn)
}
