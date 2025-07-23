package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/k6deps"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd/state"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/cmd/tests"
)

// mockExecutor mocks commandExecutor
// Records the invocation of the run function and returns the defined error
type mockExecutor struct {
	invoked bool
	err     error
}

func (m *mockExecutor) run(_ *state.GlobalState) error {
	m.invoked = true
	return m.err
}

type mockProvisioner struct {
	invoked  bool
	executor commandExecutor
	err      error
}

func (m *mockProvisioner) provision(_ k6deps.Dependencies) (commandExecutor, error) {
	m.invoked = true
	return m.executor, m.err
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
		disableBP       bool
		k6Cmd           string
		k6Args          []string
		expectProvision bool
		provisionError  error
		expectCmdRunE   bool
		expectK6Run     bool
		k6ExecutorErr   error
		expectOsExit    int
	}{
		{
			name:            "disable binary provisioning",
			k6Cmd:           "cloud",
			disableBP:       true,
			script:          fakerTest,
			expectProvision: false,
			expectCmdRunE:   true,
			expectK6Run:     false,
			expectOsExit:    0,
		},
		{
			name:            "execute binary provisioned",
			k6Cmd:           "cloud",
			script:          fakerTest,
			expectProvision: true,
			expectCmdRunE:   false,
			expectK6Run:     true,
			expectOsExit:    0,
		},
		{
			name:            "require unsatisfied k6 version",
			k6Cmd:           "cloud",
			script:          requireUnsatisfiedK6Version,
			expectProvision: true,
			expectCmdRunE:   false,
			expectK6Run:     false,
			provisionError:  fmt.Errorf("unsatisfied version"),
			expectOsExit:    -1,
		},
		{
			name:            "require satisfied k6 version",
			k6Cmd:           "cloud",
			script:          requireSatisfiedK6Version,
			expectProvision: false,
			expectCmdRunE:   true,
			expectK6Run:     false,
			expectOsExit:    0,
		},
		{
			name:            "script with no dependencies",
			k6Cmd:           "cloud",
			script:          noDepsTest,
			expectProvision: false,
			expectCmdRunE:   true,
			expectK6Run:     false,
			expectOsExit:    0,
		},
		{
			name:            "command don't require binary provisioning",
			k6Cmd:           "version",
			expectProvision: false,
			expectCmdRunE:   true,
			expectK6Run:     false,
			expectOsExit:    0,
		},
		{
			name:            "binary provisioning is not enabled for run command",
			k6Cmd:           "run",
			script:          noDepsTest,
			expectProvision: false,
			expectCmdRunE:   true,
			expectK6Run:     false,
			expectOsExit:    0,
		},
		{
			name:            "failed binary provisioning",
			k6Cmd:           "cloud",
			script:          fakerTest,
			provisionError:  errors.New("test error"),
			expectProvision: true,
			expectCmdRunE:   false,
			expectK6Run:     false,
			expectOsExit:    -1,
		},
		{
			name:            "failed k6 execution",
			k6Cmd:           "cloud",
			script:          fakerTest,
			k6ExecutorErr:   errext.WithExitCodeIfNone(errors.New("execution failed"), 108),
			expectProvision: true,
			expectCmdRunE:   false,
			expectK6Run:     true,
			expectOsExit:    108,
		},
		{
			name:            "script in stdin (unsupported)",
			k6Cmd:           "cloud",
			k6Args:          []string{"-"},
			script:          "",
			expectProvision: false,
			expectCmdRunE:   false,
			expectK6Run:     false,
			expectOsExit:    -1,
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

			ts.CmdArgs = k6Args

			// k6deps uses os package to access files. So we need to use it in the global state
			ts.FS = afero.NewOsFs()

			// NewGlobalTestState does not set the Binary provisioning flag even if we set
			// the K6_BINARY_PROVISIONING variable in the global state, so we do it manually
			ts.Flags.BinaryProvisioning = !tc.disableBP

			// the exit code is checked by the TestGlobalState when the test ends
			ts.ExpectedExitCode = tc.expectOsExit

			cmdExecutor := mockExecutor{err: tc.k6ExecutorErr}

			// use a provisioner returning the mock provisioner
			provisioner := mockProvisioner{executor: &cmdExecutor, err: tc.provisionError}
			launcher := &launcher{
				gs:          ts.GlobalState,
				provisioner: &provisioner,
			}

			rootCommand := newRootWithLauncher(ts.GlobalState, launcher)

			// find the command to be executed
			cmd, _, err := rootCommand.cmd.Find(k6Args[1:])
			if err != nil {
				t.Fatalf("parsing args %v", err)
			}

			// replace command's the RunE function by a mock that indicates if the command was executed
			runECalled := false
			cmd.RunE = func(_ *cobra.Command, _ []string) error {
				runECalled = true
				return nil
			}

			rootCommand.execute()

			assert.Equal(t, tc.expectProvision, provisioner.invoked)
			assert.Equal(t, tc.expectCmdRunE, runECalled)
			assert.Equal(t, tc.expectK6Run, cmdExecutor.invoked)
		})
	}
}

func TestIsAnalysisRequired(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: false,
		},
		{
			name:     "only flags",
			args:     []string{"-h"},
			expected: false,
		},
		{
			name:     "command does not take script",
			args:     []string{"version"},
			expected: false,
		},
		{
			name:     "run command",
			args:     []string{"run", "script.js"},
			expected: false,
		},
		{
			name:     "cloud command",
			args:     []string{"cloud", "script.js"},
			expected: true,
		},
		{
			name:     "cloud run command",
			args:     []string{"cloud", "run", "script.js"},
			expected: true,
		},
		{
			name:     "cloud upload command",
			args:     []string{"cloud", "upload", "script.js"},
			expected: true,
		},
		{
			name:     "cloud login command",
			args:     []string{"cloud", "login"},
			expected: false,
		},
		{
			name:     "archive command",
			args:     []string{"archive", "script.js"},
			expected: true,
		},
		{
			name:     "inspect command",
			args:     []string{"inspect", "archive.tar"},
			expected: true,
		},
		{
			name:     "complex case with multiple flags",
			args:     []string{"-v", "--quiet", "cloud", "run", "-o", "output.json", "--console-output", "loadtest.log", "script.js", "--tag", "env=staging"},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			args := append([]string{"k6"}, tc.args...)
			ts := tests.NewGlobalTestState(t)
			ts.CmdArgs = args
			rootCommand := newRootCommand(ts.GlobalState)

			// find the command to be executed
			cmd, _, err := rootCommand.cmd.Find(tc.args)
			if err != nil {
				t.Fatalf("parsing args %v", err)
			}

			actual := isAnalysisRequired(cmd)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
