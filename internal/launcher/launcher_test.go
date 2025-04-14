package launcher

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

// launcherFixture mocks the functions used by the launcher and tracks their execution
type luncherFixture struct {
	provisionCalled bool
	provisionDeps   k6deps.Dependencies
	k6BinaryPath    string
	k6Versions      string
	provisionError  error
	runK6Called     bool
	runK6Error      error
	runk6ReturnCode int
	fallbackCalled  bool
}

func (f *luncherFixture) fallback(gs *state.GlobalState) {
	f.fallbackCalled = true
	gs.OSExit(0)
}

func (f *luncherFixture) provision(_ *state.GlobalState, deps k6deps.Dependencies) (string, string, error) {
	f.provisionCalled = true
	f.provisionDeps = deps
	return f.k6BinaryPath, f.k6Versions, f.provisionError
}

// function to execute k6 binary
func (f *luncherFixture) run(*state.GlobalState, string) (int, error) {
	f.runK6Called = true
	return f.runk6ReturnCode, f.runK6Error
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
		fixture         *luncherFixture
		expectLogs      []string
		expectProvision bool
		expectK6Run     bool
		expectFallback  bool
		expectOsExit    int
	}{
		{
			name:  "execute binary provisioned",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          fakerTest,
			fixture:         &luncherFixture{},
			expectProvision: true,
			expectK6Run:     true,
			expectFallback:  false,
			expectOsExit:    0,
		},
		{
			name:  "require unsatisfied k6 version",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          requireUnsatisfiedK6Version,
			fixture:         &luncherFixture{},
			expectProvision: true,
			expectK6Run:     true,
			expectFallback:  false,
			expectOsExit:    0,
		},
		{
			name:  "require satisfied k6 version",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          requireSatisfiedK6Version,
			fixture:         &luncherFixture{},
			expectProvision: false,
			expectK6Run:     false,
			expectFallback:  true,
			expectOsExit:    0,
		},
		{
			name:  "script with no dependencies",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          noDepsTest,
			fixture:         &luncherFixture{},
			expectProvision: false,
			expectK6Run:     false,
			expectFallback:  true,
			expectOsExit:    0,
		},
		{
			name:  "binary provisioning disabled",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "false",
			},
			script:          fakerTest,
			fixture:         &luncherFixture{},
			expectProvision: false,
			expectK6Run:     false,
			expectFallback:  true,
			expectOsExit:    0,
		},
		{
			name:  "command don't require binary provisioning",
			k6Cmd: "version",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "false",
			},
			fixture:         &luncherFixture{},
			expectProvision: false,
			expectK6Run:     false,
			expectFallback:  true,
			expectOsExit:    0,
		},
		{
			name:  "failed binary provisioning",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script: fakerTest,
			fixture: &luncherFixture{
				provisionError: errors.New("test error"),
			},
			expectProvision: true,
			expectFallback:  false,
			expectK6Run:     false,
			expectOsExit:    1,
		},
		{
			name:  "failed k6 execution",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script: fakerTest,
			fixture: &luncherFixture{
				runK6Error:      errors.New("error executing k6"),
				runk6ReturnCode: 108,
			},
			expectProvision: true,
			expectFallback:  false,
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
			fixture:         &luncherFixture{},
			expectProvision: false,
			expectK6Run:     false,
			expectFallback:  true,
			expectOsExit:    0,
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

			launcher := &launcher{
				gs:        ts.GlobalState,
				provision: tc.fixture.provision,
				fallback:  tc.fixture.fallback,
				run:       tc.fixture.run,
			}

			launcher.Launch()

			assert.Equal(t, tc.expectProvision, tc.fixture.provisionCalled)
			assert.Equal(t, tc.expectK6Run, tc.fixture.runK6Called)
			assert.Equal(t, tc.expectFallback, tc.fixture.fallbackCalled)

			for _, l := range tc.expectLogs {
				assert.Contains(t, ts.Stdout, l)
			}
		})
	}
}
