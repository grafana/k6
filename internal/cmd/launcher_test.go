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

	"github.com/grafana/k6deps"
	"github.com/sirupsen/logrus"
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
"use k6 = v9.99"

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

func TestLauncherViaStdin(t *testing.T) {
	t.Parallel()

	k6Args := []string{"k6", "archive", "-"}

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = k6Args

	// k6deps uses os package to access files. So we need to use it in the global state
	ts.FS = fsext.NewOsFs()

	// NewGlobalTestState does not set the Binary provisioning flag even if we set
	// the K6_BINARY_PROVISIONING variable in the global state, so we do it manually
	ts.Flags.BinaryProvisioning = true

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

			deps := k6deps.Dependencies{}
			for name, constrain := range tc.deps {
				dep, err := k6deps.NewDependency(name, constrain)
				if err != nil {
					t.Fatalf("parsing %q dependency %v", name, err)
				}
				deps[dep.Name] = dep
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

func TestGetBuildServiceURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                      string
		buildSrvURL               string
		enableCommunityExtensions bool
		expectErr                 bool
		expectedURL               string
	}{
		{
			name:                      "default build service url",
			buildSrvURL:               "https://build.srv",
			enableCommunityExtensions: false,
			expectErr:                 false,
			expectedURL:               "https://build.srv/cloud",
		},
		{
			name:                      "enable community extensions",
			buildSrvURL:               "https://build.srv",
			enableCommunityExtensions: true,
			expectErr:                 false,
			expectedURL:               "https://build.srv/oss",
		},
		{
			name:                      "invalid buildServiceURL",
			buildSrvURL:               "https://host:port",
			enableCommunityExtensions: false,
			expectErr:                 true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger := &logrus.Logger{ //nolint:forbidigo
				Out: io.Discard,
			}

			flags := state.GlobalFlags{
				BinaryProvisioning:        true,
				BuildServiceURL:           tc.buildSrvURL,
				EnableCommunityExtensions: tc.enableCommunityExtensions,
			}

			buildSrvURL, err := getBuildServiceURL(flags, logger)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedURL, buildSrvURL)
			}
		})
	}
}
