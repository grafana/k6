package cmd

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/internal/cloudapi/provisioning"
	"go.k6.io/k6/v2/internal/lib/testutils"
	cloudlog "go.k6.io/k6/v2/internal/log/cloud"
	"go.k6.io/k6/v2/lib/types"
)

func TestBuildConfigFromRuntimeConfig(t *testing.T) {
	t.Parallel()

	pushInterval := "2s"
	var pushConcurrency int32 = 5
	aggPeriod := "3s"
	aggWaitPeriod := "1s"
	var aggMinSamples int32 = 50
	var maxSamples int32 = 2000

	rc := provisioning.RuntimeConfig{
		Metrics: provisioning.MetricsConfig{
			PushURL:               "https://ingest.example.com/v1/metrics",
			PushInterval:          &pushInterval,
			PushConcurrency:       &pushConcurrency,
			AggregationPeriod:     &aggPeriod,
			AggregationWaitPeriod: &aggWaitPeriod,
			AggregationMinSamples: &aggMinSamples,
			MaxSamplesPerPackage:  &maxSamples,
		},
		TestRunToken: "test-token",
		Secrets: provisioning.SecretsConfig{
			Endpoint:     "https://secrets.example.com",
			ResponsePath: "plaintext",
		},
		Logs: provisioning.LogsConfig{
			PushURL:           "https://logs.example.com/push",
			Level:             "info",
			Limit:             900,
			PushPeriodSeconds: "3s",
			MessageMaxSize:    10000,
			AllowedLabels:     []string{"lz", "level", "test_run_id"},
		},
	}

	logger := testutils.NewLogger(t)
	cfg := buildConfigFromRuntimeConfig(logger, rc)

	// push_interval → MetricPushInterval
	assert.True(t, cfg.MetricPushInterval.Valid)
	assert.Equal(t, types.NewNullDuration(2*time.Second, true), cfg.MetricPushInterval)

	// push_concurrency → MetricPushConcurrency
	assert.True(t, cfg.MetricPushConcurrency.Valid)
	assert.Equal(t, null.IntFrom(5), cfg.MetricPushConcurrency)

	// aggregation_period → AggregationPeriod
	assert.True(t, cfg.AggregationPeriod.Valid)
	assert.Equal(t, types.NewNullDuration(3*time.Second, true), cfg.AggregationPeriod)

	// aggregation_wait_period → AggregationWaitPeriod
	assert.True(t, cfg.AggregationWaitPeriod.Valid)
	assert.Equal(t, types.NewNullDuration(1*time.Second, true), cfg.AggregationWaitPeriod)

	// max_samples_per_package → MaxTimeSeriesInBatch
	assert.True(t, cfg.MaxTimeSeriesInBatch.Valid)
	assert.Equal(t, null.IntFrom(2000), cfg.MaxTimeSeriesInBatch)

	// logs.push_url → LogsPushURL
	assert.Equal(t, null.StringFrom("https://logs.example.com/push"), cfg.LogsPushURL)

	// logs.level → LogsLevel
	assert.Equal(t, null.StringFrom("info"), cfg.LogsLevel)

	// logs.limit → LogsLimit
	assert.Equal(t, null.IntFrom(900), cfg.LogsLimit)

	// logs.push_period_seconds → LogsPushPeriod (parsed from "3s")
	assert.Equal(t, types.NewNullDuration(3*time.Second, true), cfg.LogsPushPeriod)

	// logs.message_max_size → LogsMessageMaxSize
	assert.Equal(t, null.IntFrom(10000), cfg.LogsMessageMaxSize)

	// logs.allowed_labels → LogsAllowedLabels
	assert.Equal(t, []string{"lz", "level", "test_run_id"}, cfg.LogsAllowedLabels)
}

func TestBuildConfigFromRuntimeConfig_InvalidDurationLogsWarning(t *testing.T) {
	t.Parallel()

	badDuration := "not a duration"
	rc := provisioning.RuntimeConfig{
		Metrics: provisioning.MetricsConfig{
			PushInterval: &badDuration,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	hook := testutils.NewLogHook()
	logger.AddHook(hook)

	cfg := buildConfigFromRuntimeConfig(logger, rc)

	// MetricPushInterval should be left unset (invalid) when parse fails.
	assert.False(t, cfg.MetricPushInterval.Valid, "expected MetricPushInterval to remain unset for unparseable duration")

	// A warning should have been logged.
	entries := hook.Drain()
	require.NotEmpty(t, entries, "expected at least one log entry for invalid duration")

	found := false
	for _, e := range entries {
		if e.Level == logrus.WarnLevel && e.Data["field"] == "push_interval" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a warning log entry for field push_interval")
}

func TestBuildConfigFromRuntimeConfig_InvalidLogsPushPeriodLogsWarning(t *testing.T) {
	t.Parallel()

	rc := provisioning.RuntimeConfig{
		Logs: provisioning.LogsConfig{
			PushPeriodSeconds: "not a duration",
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	hook := testutils.NewLogHook()
	logger.AddHook(hook)

	cfg := buildConfigFromRuntimeConfig(logger, rc)

	// LogsPushPeriod should be left unset (invalid) when parse fails.
	assert.False(t, cfg.LogsPushPeriod.Valid, "expected LogsPushPeriod to remain unset for unparseable duration")

	// A warning should have been logged (not an error).
	entries := hook.Drain()
	require.NotEmpty(t, entries, "expected at least one log entry for invalid duration")

	found := false
	for _, e := range entries {
		assert.NotEqual(t, logrus.ErrorLevel, e.Level, "invalid duration must warn, not error")
		if e.Level == logrus.WarnLevel && e.Data["field"] == "push_period_seconds" {
			found = true
		}
	}
	assert.True(t, found, "expected a warning log entry for field push_period_seconds")
}

func TestLogPusherConfig(t *testing.T) {
	t.Parallel()

	conf := cloudapi.Config{
		LogsPushURL:        null.StringFrom("https://logs.example.com/push"),
		TestRunToken:       null.StringFrom("scoped-token"),
		LogsLevel:          null.StringFrom("warn"),
		LogsLimit:          null.IntFrom(500),
		LogsPushPeriod:     types.NewNullDuration(2*time.Second, true),
		LogsMessageMaxSize: null.IntFrom(2048),
		LogsAllowedLabels:  []string{"lz", "test_run_id"},
	}

	got := logPusherConfig(conf, "42")

	assert.Equal(t, cloudlog.Config{
		PushURL:       "https://logs.example.com/push",
		Token:         "scoped-token",
		TestRunID:     "42",
		Level:         "warn",
		Limit:         500,
		PushPeriod:    2 * time.Second,
		MsgMaxSize:    2048,
		AllowedLabels: []string{"lz", "test_run_id"},
	}, got)
}

func TestBuildConfigFromRuntimeConfig_NilFieldsLeftUnset(t *testing.T) {
	t.Parallel()

	rc := provisioning.RuntimeConfig{
		Metrics: provisioning.MetricsConfig{
			PushURL: "https://ingest.example.com/v1/metrics",
			// All nullable fields left nil.
		},
	}

	logger := testutils.NewLogger(t)
	cfg := buildConfigFromRuntimeConfig(logger, rc)

	assert.False(t, cfg.MetricPushInterval.Valid, "MetricPushInterval should be unset when input is nil")
	assert.False(t, cfg.MetricPushConcurrency.Valid, "MetricPushConcurrency should be unset when input is nil")
	assert.False(t, cfg.AggregationPeriod.Valid, "AggregationPeriod should be unset when input is nil")
	assert.False(t, cfg.AggregationWaitPeriod.Valid, "AggregationWaitPeriod should be unset when input is nil")
	assert.False(t, cfg.MaxTimeSeriesInBatch.Valid, "MaxTimeSeriesInBatch should be unset when input is nil")
}

func TestBuildConfigFromRuntimeConfig_AggregationMinSamplesIgnored(t *testing.T) {
	t.Parallel()

	var aggMinSamples int32 = 100
	rc := provisioning.RuntimeConfig{
		Metrics: provisioning.MetricsConfig{
			AggregationMinSamples: &aggMinSamples,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	hook := testutils.NewLogHook()
	logger.AddHook(hook)

	cfg := buildConfigFromRuntimeConfig(logger, rc)

	// No Config field should have been set.
	assert.False(t, cfg.MetricPushInterval.Valid)
	assert.False(t, cfg.MetricPushConcurrency.Valid)
	assert.False(t, cfg.AggregationPeriod.Valid)
	assert.False(t, cfg.AggregationWaitPeriod.Valid)
	assert.False(t, cfg.MaxTimeSeriesInBatch.Valid)

	// No warning or error should be logged (intentional drop, not an error).
	entries := hook.Drain()
	for _, e := range entries {
		assert.NotEqual(t, logrus.WarnLevel, e.Level, "unexpected warning logged for intentionally dropped field")
		assert.NotEqual(t, logrus.ErrorLevel, e.Level, "unexpected error logged for intentionally dropped field")
	}
}
