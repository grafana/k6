package cmd

import (
	"encoding/json"
	"io"
	"math/big"
	"net/url"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/metrics"
)

func TestBuiltinOutputString(t *testing.T) {
	t.Parallel()
	exp := []string{
		"cloud", "csv", "datadog", "experimental-prometheus-rw",
		"influxdb", "json", "kafka", "statsd",
		"experimental-opentelemetry", "opentelemetry",
		"summary",
	}
	assert.Equal(t, exp, builtinOutputStrings())
}

func buildTestWithPushRefID(t *testing.T, refID, token, host string) *loadedAndConfiguredTest {
	t.Helper()

	src := &loader.SourceData{
		URL:  &url.URL{Path: "test.js"},
		Data: []byte(`export default function() {}`),
	}

	scenario := executor.NewSharedIterationsConfig(lib.DefaultScenarioName)
	scenarios := lib.ScenarioConfigs{
		lib.DefaultScenarioName: scenario,
	}

	seg, err := lib.NewExecutionSegment(big.NewRat(0, 1), big.NewRat(1, 1))
	require.NoError(t, err)

	seq, err := lib.NewExecutionSegmentSequence(seg)
	require.NoError(t, err)

	cloudCfgJSON := mustMarshalCloudConfig(t, cloudapi.Config{
		PushRefID:             null.StringFrom(refID),
		Token:                 null.StringFrom(token),
		Host:                  null.StringFrom(host),
		MetricPushConcurrency: null.IntFrom(1),
		MaxTimeSeriesInBatch:  null.IntFrom(1),
		Name:                  null.StringFrom("test"),
	})

	return &loadedAndConfiguredTest{
		loadedTest: &loadedTest{
			source: src,
			preInitState: &lib.TestPreInitState{
				RuntimeOptions: lib.RuntimeOptions{
					Env: map[string]string{},
				},
			},
		},
		derivedConfig: Config{
			Options: lib.Options{
				SystemTags: metrics.NewSystemTagSet(
					metrics.TagName,
					metrics.TagStatus,
					metrics.TagMethod,
					metrics.TagGroup,
					metrics.TagCheck,
					metrics.TagError,
				),
				Scenarios:                scenarios,
				ExecutionSegment:         seg,
				ExecutionSegmentSequence: &seq,
			},
			Collectors: map[string]json.RawMessage{
				builtinOutputCloud.String(): cloudCfgJSON,
			},
		},
	}
}

func mustMarshalCloudConfig(t *testing.T, c cloudapi.Config) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(c)
	require.NoError(t, err)
	return b
}

func TestCreateCloudTest_PushRefID_SkipsCreateTestRun(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	gs := &state.GlobalState{
		Env:    map[string]string{},
		Logger: logger,
	}

	// Deliberately invalid token and unreachable host.
	// If CreateTestRun is ever called, the dial to 127.0.0.1:0
	// fails immediately — proving the early-return was skipped.
	test := buildTestWithPushRefID(t, "99999", "invalid-token", "https://127.0.0.1:0")

	err := createCloudTest(gs, test)

	// If PushRefID early-return works → no network call → no error.
	// If CreateTestRun is called (regression) → connection refused → error.
	require.NoError(t, err, "CreateTestRun must not be called when PushRefID is set")
	assert.Equal(t, "99999", test.preInitState.RuntimeOptions.Env[testRunIDKey])
}

func TestCreateCloudTest_NoPushRefID_CallsCreateTestRun(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	gs := &state.GlobalState{
		Env:    map[string]string{},
		Logger: logger,
	}

	// No PushRefID set → CreateTestRun SHOULD be called.
	// Using unreachable host ensures deterministic failure if called.
	test := buildTestWithPushRefID(t, "", "dummy-token", "https://127.0.0.1:0")

	test.derivedConfig.NoArchiveUpload = null.BoolFrom(true)

	err := createCloudTest(gs, test)

	require.Error(t, err, "CreateTestRun should be called when PushRefID is not set")
	require.Contains(t, err.Error(), "127.0.0.1", "expected a network error reaching CreateTestRun, not an earlier failure")
}
