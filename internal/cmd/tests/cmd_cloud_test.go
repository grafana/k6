package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"go.k6.io/k6/v2/errext/exitcodes"
	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/cloudapi/v6/v6test"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib/fsext"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudNoArgsShowsHelp(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "cloud"}
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, "Run and manage Grafana Cloud tests", "expected help text to be shown")
	assert.NotContains(t, stdout, "--vus", "run flags belong to `k6 cloud run`, not to `k6 cloud`")
}

type setupCommandFunc func(cliFlags []string) []string

func runCloudTests(t *testing.T, setupCmd setupCommandFunc) {
	t.Run("TestCloudUserNotAuthenticated", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		ts.ExpectedExitCode = -1 // TODO: use a more specific exit code?
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `access token not configured`)
	})

	t.Run("TestCloudUnauthorizedToken", func(t *testing.T) {
		t.Parallel()

		var reached atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reached.Store(true)
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte(`{"error":{"code":"error","message":"Invalid token"}}`))
			assert.NoError(t, err)
		}))
		defer srv.Close()

		ts := NewGlobalTestState(t)
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"),
			[]byte("export let options = { cloud: { projectID: 1 } };\nexport default function() {}"), 0o644))
		ts.CmdArgs = setupCmd([]string{"--verbose", "--log-output=stdout"})
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
		ts.Env["K6_CLOUD_TOKEN"] = "invalid-or-expired-token"
		ts.Env["K6_CLOUD_STACK_ID"] = "1"
		ts.ExpectedExitCode = -1

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		assert.True(t, reached.Load(), "mock server handler should have been reached")
		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "(401/error) Invalid token")
		assert.Contains(t, stdout, "Verify the active Grafana Cloud token (K6_CLOUD_TOKEN, options.cloud.token, or credentials saved by k6 cloud login)")
	})

	t.Run("TestCloudStackNotConfigured", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil)
		delete(ts.Env, "K6_CLOUD_STACK_ID")
		ts.ExpectedExitCode = -1
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `stack ID not configured`)
	})

	// TODO: Remove after we remove K6_CLOUD_HOST_V6.
	t.Run("TestCloudV6ClientUsesV6Host", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil)
		ts.Env["K6_CLOUD_HOST"] = "http://wrong-host"
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		require.NotContains(t, stdout, "wrong-host", "v6 client should use K6_CLOUD_HOST_V6, not K6_CLOUD_HOST")
	})

	t.Run("TestCloudLoggedInWithScriptToken", func(t *testing.T) {
		t.Parallel()

		script := `
		export let options = {
			cloud: {
				token: "asdf",
				name: "my load test",
				projectID: 124,
				note: 124,
			}
		};
		export default function() {};
	`

		ts := getSimpleCloudTestState(t, []byte(script), setupCmd, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.NotContains(t, stdout, `not logged in`)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://stack.grafana.com/a/k6-app/runs/123`)
		assert.Contains(t, stdout, `test status: Finished`)
	})

	t.Run("TestCloudExitOnRunning", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--exit-on-running", "--log-output=stdout"},
			v6test.Progress(cloudapiv6.StatusRunning, v6test.ResultNone))
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://stack.grafana.com/a/k6-app/runs/123`)
		assert.Contains(t, stdout, `test status: Running`)
	})

	t.Run("TestCloudExitOnRunningEnv", func(t *testing.T) {
		t.Parallel()

		// Same as TestCloudExitOnRunning, but driven by the environment
		// variable instead of the CLI flag. Without the override taking
		// effect, the command would keep polling the mock server forever,
		// since its progress is always "Running".
		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--log-output=stdout"},
			v6test.Progress(cloudapiv6.StatusRunning, v6test.ResultNone))
		ts.Env["K6_EXIT_ON_RUNNING"] = "true"
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://stack.grafana.com/a/k6-app/runs/123`)
		assert.Contains(t, stdout, `test status: Running`)
	})

	t.Run("TestCloudExitOnRunningFlagOverridesEnv", func(t *testing.T) {
		t.Parallel()

		// An explicitly set CLI flag takes precedence over the environment
		// variable. If the "false" from the environment won instead, the
		// command would poll the always-"Running" mock server forever.
		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--exit-on-running", "--log-output=stdout"},
			v6test.Progress(cloudapiv6.StatusRunning, v6test.ResultNone))
		ts.Env["K6_EXIT_ON_RUNNING"] = "false"
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `test status: Running`)
	})

	t.Run("TestCloudExitOnRunningInvalidEnv", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil)
		ts.Env["K6_EXIT_ON_RUNNING"] = "invalid"
		ts.ExpectedExitCode = -1
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `parsing K6_EXIT_ON_RUNNING returned an error`)
	})

	t.Run("TestCloudURLFromStartResponse", func(t *testing.T) {
		t.Parallel()

		// v6 returns the run URL in the start response (no ConfigOverride).
		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: cloud")
		assert.Contains(t, stdout, "output: https://stack.grafana.com/a/k6-app/runs/123")
		assert.Contains(t, stdout, `test status: Finished`)
	})

	t.Run("TestCloudThresholdsHaveFailed", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil,
			v6test.Progress(cloudapiv6.StatusCompleted, cloudapiv6.ResultFailed))
		ts.ExpectedExitCode = int(exitcodes.ThresholdsHaveFailed)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `Thresholds have been crossed`)
	})

	t.Run("TestCloudAbortedThreshold", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil,
			v6test.Progress(cloudapiv6.StatusAborted, cloudapiv6.ResultFailed))
		ts.ExpectedExitCode = int(exitcodes.ThresholdsHaveFailed)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `Thresholds have been crossed`)
	})

	t.Run("TestCloudAbortedByUser", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil,
			v6test.AbortedByUserProgress("user@example.com"))
		ts.ExpectedExitCode = int(exitcodes.CloudTestRunFailed)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `test status: Aborted (by user)`)
	})
}

func cloudTestStartSimple(tb testing.TB, testRunID int) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		resp.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(resp, `{
			"reference_id": "%d",
			"test_run_token": "mock-test-run-token",
			"secrets_config": {
				"endpoint": "https://mock-secrets.example.com/{key}",
				"response_path": "plaintext"
			}
		}`, testRunID)
		assert.NoError(tb, err)
	})
}

func getSimpleCloudTestState(t *testing.T, script []byte, setupCmd setupCommandFunc, cliFlags []string, progressCallback func() *cloudapiv6.TestProgress) *GlobalTestState {
	if script == nil {
		script = []byte("export let options = { cloud: { projectID: 1 } };\nexport default function() {}")
	}

	if cliFlags == nil {
		cliFlags = []string{"--verbose", "--log-output=stdout"}
	}

	srv := v6test.NewServer(t, v6test.Config{
		ProgressCallback: progressCallback,
	})

	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), script, 0o644))
	ts.CmdArgs = setupCmd(cliFlags)
	ts.Env["K6_SHOW_CLOUD_LOGS"] = "false" // no mock for the logs yet
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
	ts.Env["K6_CLOUD_TOKEN"] = "foo" // doesn't matter, we mock the cloud
	ts.Env["K6_CLOUD_STACK_ID"] = "1"

	return ts
}
