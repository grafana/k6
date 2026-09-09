package tests

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
	"go.k6.io/k6/v2/lib/fsext"
)

const scenariosScript = `
	import exec from 'k6/execution';
	export const options = {
		cloud: { name: 'pick', projectID: 123456 },
		vus: 1, duration: '1ms', iterations: 1, stages: [],
		thresholds: {
			iterations: ['count>0'],
			'iterations{scenario:api}': ['count>0'],
			'iterations{scenario:db}': ['count>0']
		},
		scenarios: {
			ui:  { executor: 'per-vu-iterations', vus: 2, iterations: 1 },
			api: { executor: 'shared-iterations', vus: 1, iterations: 2 },
			db:  { executor: 'shared-iterations', vus: 1, iterations: 1 }
		}
	};
	export default function() { console.log('ran ' + exec.scenario.name); }
`

func TestScenariosFilterArchive(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		tar  func(t *testing.T) []byte
	}{
		{
			name: "local archive",
			tar: func(t *testing.T) []byte {
				return buildArchive(t, scenariosScript, "--scenario", "ui,api")
			},
		},
		{
			name: "cloud run",
			tar: func(t *testing.T) []byte {
				_, data := uploadAndCaptureArchive(t,
					[]string{"k6", "cloud", "run", "--scenario", "ui,api", "test.js"}, nil, scenariosScript)
				return data
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tarData := tt.tar(t)
			arc, err := lib.ReadArchive(bytes.NewReader(tarData))
			require.NoError(t, err)

			require.Len(t, arc.Options.Scenarios, 2)
			assert.NotContains(t, arc.Options.Scenarios, "db")

			ui, ok := arc.Options.Scenarios["ui"].(executor.PerVUIterationsConfig)
			require.True(t, ok)
			assert.Equal(t, null.IntFrom(2), ui.VUs)
			assert.Equal(t, null.IntFrom(1), ui.Iterations)
			assert.NotContains(t, arc.Options.Thresholds, "iterations{scenario:db}")
			assert.Contains(t, arc.Options.Thresholds, "iterations")
			assert.Contains(t, arc.Options.Thresholds, "iterations{scenario:api}")
			assert.False(t, arc.Options.VUs.Valid)
			assert.False(t, arc.Options.Duration.Valid)
			assert.False(t, arc.Options.Iterations.Valid)
			assert.Nil(t, arc.Options.Stages)

			ts := NewGlobalTestState(t)
			tarPath := filepath.Join(ts.Cwd, "archive.tar")
			require.NoError(t, fsext.WriteFile(ts.FS, tarPath, tarData, 0o644))
			ts.CmdArgs = []string{"k6", "run", "--log-output=stdout", tarPath}
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			stdout := ts.Stdout.String()
			assert.Equal(t, 2, strings.Count(stdout, "ran ui"))
			assert.Equal(t, 2, strings.Count(stdout, "ran api"))
			assert.NotContains(t, stdout, "ran db")
			assert.NotContains(t, stdout, "ran default")
			assert.NotContains(t, stdout, "--scenario skipped thresholds")
		})
	}
}

func TestRunScenarios(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, scenariosScript,
		[]string{"--log-output=stdout", "--log-format=raw", "--scenario", "api"}, 0)
	ts.Env["K6_ITERATIONS"] = "1"
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Equal(t, 2, strings.Count(stdout, "ran api"))
	assert.NotContains(t, stdout, "ran ui")
	assert.NotContains(t, stdout, "ran db")
	assert.NotContains(t, stdout, "ran default")
	assert.Contains(t, stdout, `--scenario overrode vus, duration, iterations, stages in "script" configuration`)
	assert.Contains(t, stdout, `--scenario overrode iterations in "environment" configuration`)
	assert.Contains(t, stdout, "--scenario skipped thresholds for excluded scenarios: iterations{scenario:db}; "+
		"these thresholds remain skipped even if another scenario emits matching tags")
	assert.Equal(t, 1, strings.Count(stdout, "--scenario skipped thresholds"))
}

func TestScenarioThresholdValidation(t *testing.T) {
	t.Parallel()

	script := strings.Replace(scenariosScript,
		"'iterations{scenario:db}': ['count>0']", "'iterations{scenario:db}': ['invalid']", 1)
	for _, tt := range []struct {
		noThresholds string
		exitCode     exitcodes.ExitCode
	}{
		{"false", exitcodes.InvalidConfig},
		{"true", 0},
	} {
		t.Run(tt.noThresholds, func(t *testing.T) {
			t.Parallel()
			ts := getSingleFileTestState(t, script,
				[]string{"--scenario", "api", "--no-thresholds=" + tt.noThresholds}, tt.exitCode)
			cmd.ExecuteWithGlobalState(ts.GlobalState)
		})
	}
}

func TestRunScenariosOnce(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, scenariosScript,
		[]string{"--log-output=stdout", "--scenario", "ui,api", "--once"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Equal(t, 1, strings.Count(stdout, "ran ui"))
	assert.Equal(t, 1, strings.Count(stdout, "ran api"))
	assert.NotContains(t, stdout, "ran db")
}

func TestRunRejectsEmptyScenarios(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, scenariosScript,
		[]string{"--log-output=stdout", "--scenario="}, exitcodes.InvalidConfig)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, "requires at least one scenario name")
	assert.NotContains(t, stdout, "ran ")
}
