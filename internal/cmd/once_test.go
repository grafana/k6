package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/internal/cmd/tests"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
	"go.k6.io/k6/v2/lib/fsext"
	"go.k6.io/k6/v2/lib/types"
)

func TestApplyOncePreservesScenarioName(t *testing.T) {
	t.Parallel()

	opts := lib.Options{Scenarios: lib.ScenarioConfigs{
		"ui":  executor.NewConstantVUsConfig("ui"),
		"api": executor.NewConstantVUsConfig("api"),
	}}

	res, err := applyOnce(opts)
	require.NoError(t, err)
	require.Len(t, res.Scenarios, 2)

	for _, name := range []string{"ui", "api"} {
		sc, ok := res.Scenarios[name].(executor.SharedIterationsConfig)
		require.True(t, ok)
		assert.Equal(t, name, sc.GetName())
		assert.Equal(t, null.IntFrom(1), sc.VUs)
		assert.Equal(t, null.IntFrom(1), sc.Iterations)
	}
}

func mustExtractSingleSharedIterScenario(t *testing.T, opts lib.Options) executor.SharedIterationsConfig {
	t.Helper()

	require.Len(t, opts.Scenarios, 1)
	for _, sc := range opts.Scenarios {
		shared, ok := sc.(executor.SharedIterationsConfig)
		require.True(t, ok)
		return shared
	}
	return executor.SharedIterationsConfig{}
}

func TestApplyOncePreservesScenarioExec(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		exec null.String
	}{
		{"custom", null.StringFrom("api")},
		{"unset", null.String{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := executor.NewSharedIterationsConfig("ui")
			src.Exec = tt.exec
			opts := lib.Options{Scenarios: lib.ScenarioConfigs{"ui": src}}

			res, err := applyOnce(opts)
			require.NoError(t, err)
			assert.Equal(t, tt.exec, mustExtractSingleSharedIterScenario(t, res).Exec)
		})
	}
}

func TestApplyOncePreservesScenarioFields(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		setup func(*executor.SharedIterationsConfig)
		check func(*testing.T, executor.SharedIterationsConfig)
	}{
		{
			name:  "env",
			setup: func(s *executor.SharedIterationsConfig) { s.Env = map[string]string{"GREETING": "hi"} },
			check: func(t *testing.T, s executor.SharedIterationsConfig) {
				t.Helper()
				assert.Equal(t, map[string]string{"GREETING": "hi"}, s.Env)
			},
		},
		{
			name:  "tags",
			setup: func(s *executor.SharedIterationsConfig) { s.Tags = map[string]string{"team": "core"} },
			check: func(t *testing.T, s executor.SharedIterationsConfig) {
				t.Helper()
				assert.Equal(t, map[string]string{"team": "core"}, s.Tags)
			},
		},
		{
			name: "options",
			setup: func(s *executor.SharedIterationsConfig) {
				s.Options = &lib.ScenarioOptions{Browser: map[string]any{"type": "chromium"}}
			},
			check: func(t *testing.T, s executor.SharedIterationsConfig) {
				t.Helper()
				assert.Equal(t, &lib.ScenarioOptions{Browser: map[string]any{"type": "chromium"}}, s.Options)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := executor.NewSharedIterationsConfig("ui")
			tt.setup(&src)
			opts := lib.Options{Scenarios: lib.ScenarioConfigs{"ui": src}}

			res, err := applyOnce(opts)
			require.NoError(t, err)

			sc := mustExtractSingleSharedIterScenario(t, res)
			tt.check(t, sc)
		})
	}
}

func TestGetConsolidatedConfigOnceDropsLowerLayerShortcuts(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	require.NoError(t, ts.FS.MkdirAll(filepath.Dir(ts.Flags.ConfigFilePath), 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, ts.Flags.ConfigFilePath,
		[]byte(`{"vus": 7, "stages": [{"duration": "10s", "target": 5}]}`), 0o644))
	ts.Env["K6_ITERATIONS"] = "5"

	segment, err := lib.NewExecutionSegmentFromString("0:1/2")
	require.NoError(t, err)
	runnerOpts := lib.Options{
		Duration:         types.NullDurationFrom(time.Second),
		ExecutionSegment: segment,
	}

	conf, err := getConsolidatedConfig(ts.GlobalState, Config{once: true}, runnerOpts, nil)
	require.NoError(t, err)

	assert.False(t, conf.VUs.Valid)
	assert.False(t, conf.Duration.Valid)
	assert.False(t, conf.Iterations.Valid)
	assert.Nil(t, conf.Stages)
	assert.Nil(t, conf.ExecutionSegment)
}

func TestOncePreservesScenarioAcrossLayers(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	require.NoError(t, ts.FS.MkdirAll(filepath.Dir(ts.Flags.ConfigFilePath), 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, ts.Flags.ConfigFilePath, []byte(`{"scenarios": {
		"fromfile": { "executor": "constant-vus", "vus": 3, "duration": "2s", "exec": "api",
			"env": { "GREETING": "hi" }, "tags": { "team": "core" },
			"options": { "browser": { "type": "chromium" } } }
	}}`), 0o644))
	ts.Env["K6_ITERATIONS"] = "5"

	runnerOpts := lib.Options{Duration: types.NullDurationFrom(time.Second)}

	conf, err := getConsolidatedConfig(ts.GlobalState, Config{once: true}, runnerOpts, nil)
	require.NoError(t, err)

	opts, err := applyOnce(conf.Options)
	require.NoError(t, err)

	sc := mustExtractSingleSharedIterScenario(t, opts)
	assert.Equal(t, "fromfile", sc.GetName())
	assert.Equal(t, null.IntFrom(1), sc.VUs)
	assert.Equal(t, null.IntFrom(1), sc.Iterations)
	assert.Equal(t, null.StringFrom("api"), sc.Exec)
	assert.Equal(t, map[string]string{"GREETING": "hi"}, sc.Env)
	assert.Equal(t, map[string]string{"team": "core"}, sc.Tags)
	assert.Equal(t, &lib.ScenarioOptions{Browser: map[string]any{"type": "chromium"}}, sc.Options)
}
