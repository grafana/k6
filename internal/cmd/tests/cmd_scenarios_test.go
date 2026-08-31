package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
)

const scenariosScript = `
	import exec from 'k6/execution';
	export const options = {
		cloud: { name: 'pick', projectID: 123456 },
		scenarios: {
			ui:  { executor: 'constant-vus', vus: 3, duration: '2s' },
			api: { executor: 'shared-iterations', vus: 2, iterations: 4 },
			db:  { executor: 'constant-vus', vus: 1, duration: '1s' }
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

			arc, err := lib.ReadArchive(bytes.NewReader(tt.tar(t)))
			require.NoError(t, err)

			require.Len(t, arc.Options.Scenarios, 2)
			assert.NotContains(t, arc.Options.Scenarios, "db")

			ui, ok := arc.Options.Scenarios["ui"].(executor.ConstantVUsConfig)
			require.True(t, ok)
			assert.Equal(t, null.IntFrom(3), ui.VUs)
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
