package cmd

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/k6deps"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/cmd/tests"
)

type mockRunner struct {
	invoked bool
	rc      int
}

func (m *mockRunner) run(gs *state.GlobalState) {
	m.invoked = true
	gs.OSExit(m.rc)
}

const (
	fakerTest = `
import { Faker } from "k6/x/faker";

const faker = new Faker(11);

export default function () {
  console.log(faker.person.firstName());
}
`

	noDepsTest = `
import http from 'k6/http';
import { sleep, check } from 'k6';

export default function() {
  let res = http.get('https://quickpizza.grafana.com');
  check(res, { "status is 200": (res) => res.status === 200 });
  sleep(1);
}
`

	requireUnsatisfiedK6Version = `
"use k6 = v0.99"

import { sleep, check } from 'k6';

export default function() {
  let res = http.get('https://quickpizza.grafana.com');
  check(res, { "status is 200": (res) => res.status === 200 });
  sleep(1);
}
`
	// FIXME: when the build version is a prerelease (e.g v1.0.0-rc1), k6deps fails to parse this pragma
	// and creates an invalid constrain that is ignored by the test.
	// see https://github.com/grafana/k6deps/issues/91
	requireSatisfiedK6Version = `
"use k6 = v` + build.Version + `";
import { sleep, check } from 'k6';

export default function() {
  let res = http.get('https://quickpizza.grafana.com');
  check(res, { "status is 200": (res) => res.status === 200 });
  sleep(1);
}
`
)

func TestLauncherLaunch(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		script          string
		k6Env           map[string]string
		k6Cmd           string
		k6Args          []string
		expectProvision bool
		provisionError  error
		expectK6Run     bool
		expectDefault   bool
		k6ReturnCode    int
		expectOsExit    int
	}{
		{
			name:  "execute binary provisioned",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          fakerTest,
			expectProvision: true,
			expectK6Run:     true,
			expectDefault:   false,
			expectOsExit:    0,
		},
		{
			name:  "require unsatisfied k6 version",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          requireUnsatisfiedK6Version,
			expectProvision: true,
			expectK6Run:     true,
			expectDefault:   false,
			expectOsExit:    0,
		},
		{
			name:  "require satisfied k6 version",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          requireSatisfiedK6Version,
			expectProvision: false,
			expectK6Run:     false,
			expectDefault:   true,
			expectOsExit:    0,
		},
		{
			name:  "script with no dependencies",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          noDepsTest,
			expectProvision: false,
			expectK6Run:     false,
			expectDefault:   true,
			expectOsExit:    0,
		},
		{
			name:  "binary provisioning disabled",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "false",
			},
			script:          fakerTest,
			expectProvision: false,
			expectK6Run:     false,
			expectDefault:   true,
			expectOsExit:    0,
		},
		{
			name:  "command don't require binary provisioning",
			k6Cmd: "version",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "false",
			},
			expectProvision: false,
			expectK6Run:     false,
			expectDefault:   true,
			expectOsExit:    0,
		},
		{
			name:  "failed binary provisioning",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          fakerTest,
			provisionError:  errors.New("test error"),
			expectProvision: true,
			expectDefault:   false,
			expectK6Run:     false,
			expectOsExit:    1,
		},
		{
			name:  "failed k6 execution",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          fakerTest,
			k6ReturnCode:    108,
			expectProvision: true,
			expectDefault:   false,
			expectK6Run:     true,
			expectOsExit:    108,
		},
		{
			name:   "script in stdin",
			k6Cmd:  "run",
			k6Args: []string{"-"},
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          "",
			expectProvision: false,
			expectK6Run:     false,
			expectDefault:   false,
			expectOsExit:    1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)

			k6Args := append([]string{"k6"}, tc.k6Cmd)
			k6Args = append(k6Args, tc.k6Args...)

			// create tmp file with the script if specified
			if len(tc.script) > 0 {
				scriptPath := filepath.Join(t.TempDir(), "script.js")
				if err := os.WriteFile(scriptPath, []byte(tc.script), 0o600); err != nil { //nolint:forbidigo
					t.Fatalf("test setup: creating script file %v", err)
				}
				k6Args = append(k6Args, scriptPath)
			}

			ts.GlobalState.CmdArgs = k6Args
			maps.Copy(ts.GlobalState.Env, tc.k6Env)

			// k6deps uses os package to access files. So we need to use it in the global state
			ts.GlobalState.FS = afero.NewOsFs()

			// NewGlobalTestState does not set the Binary provisioning flag, set it manually
			ts.GlobalState.Flags.BinaryProvisioning = (tc.k6Env["K6_BINARY_PROVISIONING"] == "true")

			// the exit code is checked by the TestGlobalState when the test ends
			ts.ExpectedExitCode = tc.expectOsExit

			defaultRunner := &mockRunner{}
			provisionRunner := &mockRunner{rc: tc.k6ReturnCode}
			provisionCalled := false
			launcher := &Launcher{
				gs: ts.GlobalState,
				provision: func(_ *state.GlobalState, _ k6deps.Dependencies) (binaryRunner, error) {
					provisionCalled = true
					return provisionRunner, tc.provisionError
				},
				binaryRunner: defaultRunner,
			}

			launcher.Launch()

			assert.Equal(t, tc.expectProvision, provisionCalled)
			assert.Equal(t, tc.expectK6Run, provisionRunner.invoked)
			assert.Equal(t, tc.expectDefault, defaultRunner.invoked)
		})
	}
}

func TestScriptArg(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		args      []string
		expected  string
		hasScript bool
	}{
		{
			name:      "empty args",
			args:      []string{},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "only flags",
			args:      []string{"-v", "--verbose"},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "invalid command",
			args:      []string{"invalid", "script.js"},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "run with script at end",
			args:      []string{"run", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with script and flags",
			args:      []string{"run", "script.js", "-v"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with flags before script",
			args:      []string{"run", "-v", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with verbose flag before script",
			args:      []string{"run", "--verbose", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with flag with value before script",
			args:      []string{"run", "--console-output", "loadtest.log", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with script before flag with value",
			args:      []string{"run", "script.js", "--console-output", "loadtest2.log"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "cloud run with script",
			args:      []string{"cloud", "run", "archive.tar"},
			expected:  "archive.tar",
			hasScript: true,
		},
		{
			name:      "cloud run with script and flags",
			args:      []string{"cloud", "run", "archive.tar", "-v"},
			expected:  "archive.tar",
			hasScript: true,
		},
		{
			name:      "cloud with script and flags",
			args:      []string{"cloud", "archive.tar", "-v"},
			expected:  "archive.tar",
			hasScript: true,
		},
		{
			name:      "cloud with flags and script",
			args:      []string{"cloud", "--console-output", "loadtest.log", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "cloud with script and flags with value",
			args:      []string{"cloud", "script.js", "--console-output", "loadtest2.log"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "flags before command",
			args:      []string{"-v", "run", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "complex case with multiple flags",
			args:      []string{"-v", "--quiet", "run", "-o", "output.json", "--console-output", "loadtest.log", "script.js", "--tag", "env=staging"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "no script file",
			args:      []string{"run", "-v", "--quiet"},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "non-script file",
			args:      []string{"run", "notascript.txt"},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "ts extension",
			args:      []string{"run", "script.ts"},
			expected:  "script.ts",
			hasScript: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			script, hasScript := scriptNameFromArgs(tc.args)

			assert.Equal(t, tc.hasScript, hasScript)
			assert.Equal(t, tc.expected, script)
		})
	}
}
