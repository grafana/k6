package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/lib/fsext"
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

func (m *mockProvisioner) provision(_ map[string]string) (commandExecutor, error) {
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
"use k6 = v9.99"

import { sleep, check } from 'k6';

export default function() {
  let res = http.get('https://quickpizza.grafana.com');
  check(res, { "status is 200": (res) => res.status === 200 });
  sleep(1);
}
`
	// FIXME: when the build version is a prerelease (e.g v1.0.0-rc1), k6deps fails to parse this pragma
	// and creates an invalid constraint that is ignored by the test.
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
		name               string
		script             string
		disableAER         bool
		k6Cmd              string
		k6Args             []string
		expectProvision    bool
		provisionError     error
		expectCmdRunE      bool
		expectK6Run        bool
		k6ExecutorErr      error
		expectOsExit       int
		expectedDepsString string
	}{
		{
			name:            "disable automatic extension resolution",
			k6Cmd:           "cloud",
			disableAER:      true,
			script:          fakerTest,
			expectProvision: false,
			expectCmdRunE:   true,
			expectK6Run:     false,
			expectOsExit:    0,
		},
		{
			name:               "execute binary provisioned",
			k6Cmd:              "cloud",
			script:             fakerTest,
			expectProvision:    true,
			expectCmdRunE:      false,
			expectK6Run:        true,
			expectOsExit:       0,
			expectedDepsString: "k6*;k6/x/faker*",
		},
		{
			name:               "require unsatisfied k6 version",
			k6Cmd:              "cloud",
			script:             requireUnsatisfiedK6Version,
			expectProvision:    true,
			expectCmdRunE:      false,
			expectK6Run:        false,
			provisionError:     fmt.Errorf("unsatisfied version"),
			expectOsExit:       -1,
			expectedDepsString: "k6=v9.99",
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
			name:            "script with no extension dependencies",
			k6Cmd:           "cloud",
			script:          noDepsTest,
			expectProvision: false,
			expectCmdRunE:   true,
			expectK6Run:     false,
			expectOsExit:    0,
		},
		{
			name:            "command don't require automatic extension resolution",
			k6Cmd:           "version",
			expectProvision: false,
			expectCmdRunE:   true,
			expectK6Run:     false,
			expectOsExit:    0,
		},
		{
			name:            "automatic extension resolution not enabled for run command",
			k6Cmd:           "run",
			script:          noDepsTest,
			expectProvision: false,
			expectCmdRunE:   true,
			expectK6Run:     false,
			expectOsExit:    0,
		},
		{
			name:               "failed binary provisioning",
			k6Cmd:              "cloud",
			script:             fakerTest,
			provisionError:     errors.New("test error"),
			expectProvision:    true,
			expectCmdRunE:      false,
			expectK6Run:        false,
			expectOsExit:       -1,
			expectedDepsString: "k6*;k6/x/faker*",
		},
		{
			name:               "failed k6 execution",
			k6Cmd:              "cloud",
			script:             fakerTest,
			k6ExecutorErr:      errext.WithExitCodeIfNone(errors.New("execution failed"), 108),
			expectProvision:    true,
			expectCmdRunE:      false,
			expectK6Run:        true,
			expectOsExit:       108,
			expectedDepsString: "k6*;k6/x/faker*",
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
			ts.FS = fsext.NewOsFs()

			// NewGlobalTestState does not set the AutoExtensionResolution flag even if we set
			// the K6_AUTO_EXTENSION_RESOLUTION variable in the global state, so we do it manually
			ts.Flags.AutoExtensionResolution = !tc.disableAER

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
			if tc.expectK6Run {
				assert.Contains(t, ts.Stderr.String(), "deps=\""+tc.expectedDepsString+"\"")
			}
		})
	}
}

func TestLauncherViaStdin(t *testing.T) {
	t.Parallel()

	k6Args := []string{"k6", "archive", "-"}

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = k6Args

	// k6deps uses os package to access files. So we need to use it in the global state
	ts.FS = fsext.NewOsFs()

	// NewGlobalTestState does not set the AutoExtensionResolution flag even if we set
	// the K6_AUTO_EXTENSION_RESOLUTION variable in the global state, so we do it manually
	ts.Flags.AutoExtensionResolution = true

	// pass script using stdin
	stdin := bytes.NewBuffer([]byte(requireUnsatisfiedK6Version))
	ts.Stdin = stdin

	// the exit code is checked by the TestGlobalState when the test ends
	ts.ExpectedExitCode = 0

	rootCommand := newRootCommand(ts.GlobalState)
	cmdExecutor := mockExecutor{}

	// use a provider returning the mock command executor
	provider := mockProvisioner{executor: &cmdExecutor}
	launcher := &launcher{
		gs:          ts.GlobalState,
		provisioner: &provider,
	}

	// override the rootCommand launcher
	rootCommand.launcher = launcher

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

	assert.Equal(t, true, provider.invoked)
	assert.Equal(t, false, runECalled)
	assert.Equal(t, true, cmdExecutor.invoked)
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
			expected: true,
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
			// Specifically set to not be the default k6 name.
			// It asserts the scenario where a user has a custom name for the binary,
			// such as k6v1.2.2, which is useful for managing multiple installed versions.
			ts.BinaryName = "somethingelse"
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

func TestIsCustomBuildRequired(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title  string
		deps   map[string]string
		exts   []*ext.Extension
		expect bool
	}{
		{
			title:  "k6 satisfied",
			deps:   map[string]string{"k6": "=v1.0.0"},
			exts:   []*ext.Extension{},
			expect: false,
		},
		{
			title:  "k6 not satisfied",
			deps:   map[string]string{"k6": ">v1.0.0"},
			exts:   []*ext.Extension{},
			expect: true,
		},
		{
			title:  "extension not present",
			deps:   map[string]string{"k6": "*", "k6/x/faker": "*"},
			exts:   []*ext.Extension{},
			expect: true,
		},
		{
			title: "extension satisfied",
			deps:  map[string]string{"k6/x/faker": "=v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: false,
		},
		{
			title: "extension not satisfied",
			deps:  map[string]string{"k6/x/faker": ">v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: true,
		},
		{
			title: "k6 and extension satisfied",
			deps:  map[string]string{"k6": "=v1.0.0", "k6/x/faker": "=v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: false,
		},
		{
			title: "k6 satisfied, extension not satisfied",
			deps:  map[string]string{"k6": "=v1.0.0", "k6/x/faker": ">v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: true,
		},
		{
			title: "k6 not satisfied, extension satisfied",
			deps:  map[string]string{"k6": ">v1.0.0", "k6/x/faker": "=v0.4.0"},
			exts: []*ext.Extension{
				{Name: "k6/x/faker", Module: "github.com/grafana/xk6-faker", Version: "v0.4.0"},
			},
			expect: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			deps := make(map[string]*semver.Constraints)
			for name, constraint := range tc.deps {
				dep, err := k6deps.NewDependency(name, constraint)
				if err != nil {
					t.Fatalf("parsing %q dependency %v", name, err)
				}
				deps[dep.Name] = dep.Constraints
			}

			k6Version := "v1.0.0"
			required := isCustomBuildRequired(deps, k6Version, tc.exts)
			assert.Equal(t, tc.expect, required)
		})
	}
}

func TestIOFSBridgeOpen(t *testing.T) {
	t.Parallel()

	testfs := afero.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(testfs, "abasicpath/onetwo.txt", []byte(`test123`), 0o644))

	bridge := &ioFSBridge{fsext: testfs}

	// It asserts that the bridge implements io/fs.FS
	goiofs := fs.FS(bridge)
	f, err := goiofs.Open("abasicpath/onetwo.txt")
	require.NoError(t, err)
	require.NotNil(t, f)

	content, err := io.ReadAll(f)
	require.NoError(t, err)

	assert.Equal(t, "test123", string(content))
}

func TestGetProviderConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		token        string
		expectConfig k6provider.Config
	}{
		{
			name:  "no token",
			token: "",
			expectConfig: k6provider.Config{
				BuildServiceURL:  "https://ingest.k6.io/builder/api/v1",
				BinaryCacheDir:   filepath.Join(".cache", "k6", "builds"),
				BuildServiceAuth: "",
			},
		},
		{
			name:  "K6_CLOUD_TOKEN set",
			token: "K6CLOUDTOKEN",
			expectConfig: k6provider.Config{
				BuildServiceURL:  "https://ingest.k6.io/builder/api/v1",
				BinaryCacheDir:   filepath.Join(".cache", "k6", "builds"),
				BuildServiceAuth: "K6CLOUDTOKEN",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			if tc.token != "" {
				ts.Env["K6_CLOUD_TOKEN"] = tc.token
			}

			config := getProviderConfig(ts.GlobalState)

			assert.Equal(t, tc.expectConfig, config)
		})
	}
}

func TestProcessUseDirectives(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input          string
		expectedOutput map[string]string
		expectedError  string
	}{
		"nothing": {
			input: "export default function() {}",
		},
		"nothing really": {
			input: `"use k6"`,
			expectedOutput: map[string]string{
				"k6": "",
			},
		},
		"k6 pinning": {
			input: `"use k6 > 1.4.0"`,
			expectedOutput: map[string]string{
				"k6": "> 1.4.0",
			},
		},
		"a extension": {
			input: `"use k6 with k6/x/sql"`,
			expectedOutput: map[string]string{
				"k6/x/sql": "",
			},
		},
		"an extension with constraint": {
			input: `"use k6 with k6/x/sql > 1.4.0"`,
			expectedOutput: map[string]string{
				"k6/x/sql": "> 1.4.0",
			},
		},
		"complex": {
			input: `
				// something here
				"use k6 with k6/x/A"
				function a (){
					"use k6 with k6/x/B"
					let s = JSON.stringify( "use k6 with k6/x/C")
					"use k6 with k6/x/D"

					return s
				}

				export const b = "use k6 with k6/x/E"
				"use k6 with k6/x/F"

				// Here for esbuild and k6 warnings
				a()
				export default function(){}
				`,
			expectedOutput: map[string]string{
				"k6/x/A": "",
			},
		},

		"repeat": {
			input: `
				"use k6 with k6/x/A"
				"use k6 with k6/x/A"
				`,
			expectedOutput: map[string]string{
				"k6/x/A": "",
			},
		},
		"repeat with constraint first": {
			input: `
				"use k6 with k6/x/A > 1.4.0"
				"use k6 with k6/x/A"
				`,
			expectedOutput: map[string]string{
				"k6/x/A": "> 1.4.0",
			},
		},
		"constraint difference": {
			input: `
				"use k6 > 1.4.0"
				"use k6 = 1.2.3"
				`,
			expectedError: `error while parsing use directives in "name.js": already have constraint for "k6", when parsing "=1.2.3"`,
		},
		"constraint difference for extensions": {
			input: `
				"use k6 with k6/x/A > 1.4.0"
				"use k6 with k6/x/A = 1.2.3"
				`,
			expectedError: `error while parsing use directives in "name.js": already have constraint for "k6/x/A", when parsing "=1.2.3"`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			deps := make(dependencies)
			for k, v := range test.expectedOutput {
				require.NoError(t, deps.update(k, v))
			}
			if len(test.expectedError) > 0 {
				deps = nil
			}

			m, err := processUseDirectives("name.js", []byte(test.input))
			assert.EqualValues(t, deps, m)
			if len(test.expectedError) > 0 {
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}

func TestFindDirectives(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input          string
		expectedOutput []string
	}{
		"nothing": {
			input:          "export default function() {}",
			expectedOutput: nil,
		},
		"nothing really": {
			input:          `"use k6"`,
			expectedOutput: []string{"use k6"},
		},
		"multiline": {
			input: `
			"use k6 with k6/x/sql"
			"something"
			`,
			expectedOutput: []string{"use k6 with k6/x/sql", "something"},
		},
		"multiline start at beginning": {
			input: `
"use k6 with k6/x/sql"
"something"
			`,
			expectedOutput: []string{"use k6 with k6/x/sql", "something"},
		},
		"multiline comments": {
			input: `#!/bin/sh
			// here comment "hello"
"use k6 with k6/x/sql";
			/*
			"something else here as well"
			*/
	;
"something";
const l = 5
"more"
			`,
			expectedOutput: []string{"use k6 with k6/x/sql", "something"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := findDirectives([]byte(test.input))
			assert.EqualValues(t, test.expectedOutput, m)
		})
	}
}
