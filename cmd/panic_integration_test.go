package cmd

// TODO: convert this into the integration tests, once https://github.com/grafana/k6/issues/2459 will be done

import (
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cmd/tests"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/testutils"
)

// alarmist is a mock module that do a panic
type alarmist struct {
	vu modules.VU
}

var _ modules.Module = &alarmist{}

func (a *alarmist) NewModuleInstance(vu modules.VU) modules.Instance {
	return &alarmist{
		vu: vu,
	}
}

func (a *alarmist) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"panic": a.panic,
		},
	}
}

func (a *alarmist) panic(s string) {
	panic(s)
}

func init() {
	modules.Register("k6/x/alarmist", new(alarmist))
}

func TestRunScriptPanicsErrorsAndAbort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		caseName, testScript, expectedLogMessage string
	}{
		{
			caseName: "panic in the VU context",
			testScript: `
			import { panic } from 'k6/x/alarmist';

			export default function() {
				panic('hey');
				console.log('lorem ipsum');
			}
			`,
			expectedLogMessage: "a panic occurred during JS execution: hey",
		},
		{
			caseName: "panic in the init context",
			testScript: `
			import { panic } from 'k6/x/alarmist';

			panic('hey');
			export default function() {
				console.log('lorem ipsum');
			}
			`,
			expectedLogMessage: "a panic occurred during JS execution: hey",
		},
	}

	for _, tc := range testCases {
		tc := tc
		name := tc.caseName

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testFilename := "script.js"
			ts := tests.NewGlobalTestState(t)
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, testFilename), []byte(tc.testScript), 0o644))
			ts.CmdArgs = []string{"k6", "run", testFilename}

			ts.ExpectedExitCode = int(exitcodes.ScriptAborted)
			newRootCommand(ts.GlobalState).execute()

			logs := ts.LoggerHook.Drain()

			assert.True(t, testutils.LogContains(logs, logrus.ErrorLevel, tc.expectedLogMessage))
			assert.False(t, testutils.LogContains(logs, logrus.InfoLevel, "lorem ipsum"))
		})
	}
}
