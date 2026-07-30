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

// TestConfig_NewFieldsRoundTripThroughJSON also serves as the security
// regression for "scoped tokens stay in-process": the fields live on
// cloudapi.Config (which round-trips only via derivedConfig.Collectors[cloud])
// and NOT on lib.Options (which is serialised into the archive's
// metadata.json and pushed to the cloud). A new field accidentally added
// to lib.Options would be serialised into the archive — but the new
// fields here are on cloudapi.Config, so they cannot leak that way.
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

func TestConfig_NewFieldsNotPickedUpByEnvconfig(t *testing.T) {
	t.Parallel()

	// Set env vars with every plausible name a user might guess.
	envVars := map[string]string{
		"K6_CLOUD_METRICS_PUSH_URL": "foo",
		"K6_CLOUD_TEST_RUN_TOKEN":   "bar",
		"K6_CLOUD_METRICSPUSHURL":   "foo",
		"K6_CLOUD_TESTRUNTOKEN":     "bar",
	}

	config, _, err := GetConsolidatedConfig(nil, envVars, "", nil)
	require.NoError(t, err)

	assert.Equal(t, null.String{}, config.MetricsPushURL)
	assert.Equal(t, null.String{}, config.TestRunToken)
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
