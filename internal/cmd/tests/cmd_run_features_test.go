package tests

import (
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib/fsext"
)

const featuresRunScript = `
	export const options = {
		scenarios: {
			s: { executor: 'per-vu-iterations', vus: 1, iterations: 1 },
		},
	};
	export default function () {}
`

func runAndNativeHistTagged(t *testing.T, ts *GlobalTestState) []float64 {
	t.Helper()

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	jsonResults, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)

	return getSampleValues(t, jsonResults, "iterations",
		map[string]string{"k6_feature_native_histograms": "true"})
}

func TestRunWithFeatureFlag(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, featuresRunScript,
		[]string{"--out", "json=results.json", "--no-usage-report", "--features", "native-histograms"}, 0)

	tagged := runAndNativeHistTagged(t, ts)
	assert.NotEmpty(t, tagged, "iterations samples must carry the feature tag")

	assert.Equal(t, []string{"native-histograms"}, ts.Usage.Map()["features"])

	var sawExperimentalInfo bool
	for _, e := range ts.LoggerHook.Drain() {
		if e.Level == logrus.InfoLevel &&
			e.Data["feature"] == "native-histograms" &&
			e.Data["lifecycle"] == "experimental" {
			sawExperimentalInfo = true
		}
	}
	assert.True(t, sawExperimentalInfo, "expected experimental lifecycle INFO")
}

func TestRunWithoutFeatureFlag(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, featuresRunScript,
		[]string{"--out", "json=results.json", "--no-usage-report"}, 0)

	tagged := runAndNativeHistTagged(t, ts)
	assert.Empty(t, tagged, "no feature tag without activation")

	assert.Nil(t, ts.Usage.Map()["features"])

	for _, e := range ts.LoggerHook.Drain() {
		assert.NotEqual(t, "native-histograms", e.Data["feature"])
	}
}

func TestRunWithLegacyAliasEnvVar(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, featuresRunScript,
		[]string{"--out", "json=results.json", "--no-usage-report"}, 0)
	ts.Env["K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM"] = "true"

	tagged := runAndNativeHistTagged(t, ts)
	assert.NotEmpty(t, tagged, "legacy alias must activate the canonical feature")

	assert.Equal(t, []string{"native-histograms"}, ts.Usage.Map()["features"])

	var sawAliasWarn bool
	for _, e := range ts.LoggerHook.Drain() {
		if e.Level == logrus.WarnLevel && e.Data["source"] == "env_legacy_alias" {
			sawAliasWarn = true
		}
	}
	assert.True(t, sawAliasWarn, "expected legacy-alias deprecation WARN")
}

func TestRunEmptyCLIOverridesLegacyAlias(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, featuresRunScript,
		[]string{"--out", "json=results.json", "--no-usage-report", "--features", ""}, 0)
	ts.Env["K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM"] = "true"

	tagged := runAndNativeHistTagged(t, ts)
	assert.Empty(t, tagged, "empty CLI surface must override the legacy alias")
	assert.Nil(t, ts.Usage.Map()["features"])
}

func TestRunUnknownFeatureNameExitsZero(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, featuresRunScript,
		[]string{"--out", "json=results.json", "--no-usage-report", "--features", "not-a-flag"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	var sawUnknownError bool
	for _, e := range ts.LoggerHook.Drain() {
		if e.Level == logrus.ErrorLevel &&
			e.Data["feature"] == "not-a-flag" &&
			e.Data["outcome"] == "unknown" &&
			e.Data["source"] == "cli" {
			sawUnknownError = true
		}
	}
	assert.True(t, sawUnknownError, "expected unknown-name ERROR with source=cli")
	assert.Nil(t, ts.Usage.Map()["features"], "unknown name must not contribute to telemetry")
}

func TestRunActivatesFeatureFromJSONConfig(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, featuresRunScript,
		[]string{"--out", "json=results.json", "--no-usage-report"}, 0)
	require.NoError(t, ts.FS.MkdirAll(filepath.Dir(ts.Flags.ConfigFilePath), 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, ts.Flags.ConfigFilePath,
		[]byte(`{"features":["native-histograms"]}`), 0o644))

	tagged := runAndNativeHistTagged(t, ts)
	assert.NotEmpty(t, tagged, "JSON config features must activate the flag")
}
