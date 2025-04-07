package launcher

import (
	//"errors"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/k6deps"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/cmd/tests"
)

// the launcher fixture mocks the functions used by the launcher and tracks their execution
type luncherFixture struct {
	provisionCalled bool
	provisionDeps   k6deps.Dependencies
	k6BinaryPath    string
	k6Versions      string
	provisionError  error
	k6RunPath       string
	runK6Called     bool
	runK6Error      error
	runk6ReturnCode int
	fallbackCalled  bool
}

func (f *luncherFixture) fallback(gs *state.GlobalState) {
	f.fallbackCalled = true
}

func (f *luncherFixture) provision(s *state.GlobalState, deps k6deps.Dependencies) (string, string, error) {
	f.provisionCalled = true
	f.provisionDeps = deps
	return f.k6BinaryPath, f.k6Versions, f.provisionError
}

// function to execute k6 binary
func (f *luncherFixture) run(*state.GlobalState, string) (error, int) {
	f.runK6Called = true
	return f.runK6Error, f.runk6ReturnCode
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

	requireK6Version = `
"use k6 = v0.99.9"

import { sleep, check } from 'k6';

export default function() {
  let res = http.get('https://quickpizza.grafana.com');
  check(res, { "status is 200": (res) => res.status === 200 });
  sleep(1);
}
`

	requireAnyK6Version = `
"use k6 = *"

import { sleep, check } from 'k6';

export default function() {
  let res = http.get('https://quickpizza.grafana.com');
  check(res, { "status is 200": (res) => res.status === 200 });
  sleep(1);
}
`
)

func Test_Launcher(t *testing.T) {

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
			name:  "require pinned k6 version",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          requireK6Version,
			fixture:         &luncherFixture{},
			expectProvision: true,
			expectK6Run:     true,
			expectFallback:  false,
			expectOsExit:    0,
		},
		{
			name:  "require any k6 version",
			k6Cmd: "run",
			k6Env: map[string]string{
				"K6_BINARY_PROVISIONING": "true",
			},
			script:          requireAnyK6Version,
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
			name:  " command don't require binary provisioning",
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := tests.NewGlobalTestState(t)

			k6Args := append([]string{"k6"}, tc.k6Cmd)
			k6Args = append(k6Args, tc.k6Args...)

			// create tmp file with the script if specified
			if len(tc.script) > 0 {
				scriptPath := filepath.Join(t.TempDir(), "script.js")
				if err := os.WriteFile(scriptPath, []byte(tc.script), 0644); err != nil {
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

			launcher := &launcher{
				gs:        ts.GlobalState,
				provision: tc.fixture.provision,
				fallback:  tc.fixture.fallback,
				run:       tc.fixture.run,
			}

			rc := launcher.launch()

			assert.Equal(t, tc.expectProvision, tc.fixture.provisionCalled)
			assert.Equal(t, tc.expectK6Run, tc.fixture.runK6Called)
			assert.Equal(t, tc.expectFallback, tc.fixture.fallbackCalled)
			assert.Equal(t, tc.expectOsExit, rc)

			for _, l := range tc.expectLogs {
				assert.Contains(t, ts.Stdout, l)
			}
		})
	}
}
