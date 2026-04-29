package tests

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/errext/exitcodes"
	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/cloudapi/v6/v6test"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib/fsext"
)

// cloudLocalExecScript is a minimal JS script with cloud options used
// across multiple local-execution tests.
const cloudLocalExecScript = `
export const options = {
  cloud: {
      name: 'Hello k6 Cloud!',
      projectID: 123456,
  },
};

export default function() {};`

func TestK6CloudRun(t *testing.T) {
	t.Parallel()
	runCloudTests(t, setupK6CloudRunCmd)
}

func setupK6CloudRunCmd(cliFlags []string) []string {
	return append([]string{"k6", "cloud", "run"}, append(cliFlags, "test.js")...)
}

func TestCloudRunCommandIncompatibleFlags(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		cliArgs            []string
		wantStderrContains string
	}{
		{
			name:               "using --linger should be incompatible with k6 cloud run",
			cliArgs:            []string{"--linger"},
			wantStderrContains: "the --linger flag can only be used in conjunction with the --local-execution flag",
		},
		{
			name:               "using --exit-on-running should be incompatible with k6 cloud run --local-execution",
			cliArgs:            []string{"--local-execution", "--exit-on-running"},
			wantStderrContains: "the --local-execution flag is not compatible with the --exit-on-running flag",
		},
		{
			name:               "using --show-logs should be incompatible with k6 cloud run --local-execution",
			cliArgs:            []string{"--local-execution", "--show-logs"},
			wantStderrContains: "the --local-execution flag is not compatible with the --show-logs flag",
		},
		{
			name:               "using --secret-source=cloud without --local-execution should fail",
			cliArgs:            []string{"--secret-source=cloud"},
			wantStderrContains: "the 'cloud' secret source can only be used with 'k6 cloud run --local-execution'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := getSimpleCloudTestState(t, nil, setupK6CloudRunCmd, tc.cliArgs, nil)
			ts.ExpectedExitCode = int(exitcodes.InvalidConfig)
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			stderr := ts.Stderr.String()
			assert.Contains(t, stderr, tc.wantStderrContains)
		})
	}
}

func TestCloudRunLocalExecution(t *testing.T) {
	t.Parallel()

	t.Run("should upload the test archive as a default", func(t *testing.T) {
		t.Parallel()

		ts := makeTestState(t, cloudLocalExecScript, []string{"--local-execution"})
		srv := v6test.NewServer(t, v6test.Config{
			ProgressCallback: func() *cloudapiv6.TestProgress {
				return &cloudapiv6.TestProgress{Status: cloudapiv6.StatusInitializing}
			},
		})
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
		ts.Env["K6_CLOUD_STACK_ID"] = "1"

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, fmt.Sprintf("%d", v6test.DefaultTestRunID))
	})

	t.Run("does not upload the archive when --no-archive-upload is provided", func(t *testing.T) {
		t.Parallel()

		ts := makeTestState(t, cloudLocalExecScript, []string{"--local-execution", "--no-archive-upload"})
		srv := v6test.NewServer(t, v6test.Config{
			ProgressCallback: func() *cloudapiv6.TestProgress {
				return &cloudapiv6.TestProgress{Status: cloudapiv6.StatusInitializing}
			},
		})
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
		ts.Env["K6_CLOUD_STACK_ID"] = "1"

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, fmt.Sprintf("%d", v6test.DefaultTestRunID))
	})

	t.Run("the script can read the test run id to the environment", func(t *testing.T) {
		t.Parallel()

		script := `
export const options = {
  cloud: {
      name: 'Hello k6 Cloud!',
      projectID: 123456,
  },
};

export default function() {
	` + "console.log(`The test run id is ${__ENV.K6_CLOUDRUN_TEST_RUN_ID}`);" + `
};`

		ts := makeTestState(t, script, []string{"--local-execution", "--log-output=stdout"})
		srv := v6test.NewServer(t, v6test.Config{
			ProgressCallback: func() *cloudapiv6.TestProgress {
				return &cloudapiv6.TestProgress{Status: cloudapiv6.StatusInitializing}
			},
		})
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
		ts.Env["K6_CLOUD_STACK_ID"] = "1"

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, fmt.Sprintf("The test run id is %d", v6test.DefaultTestRunID))
	})

	t.Run("reuses existing test run when K6_CLOUD_PUSH_REF_ID is set", func(t *testing.T) {
		t.Parallel()

		script := `
export const options = {
  cloud: {
	  name: 'Hello k6 Cloud!',
	  projectID: 123456,
  },
};

export default function() {
    ` + "console.log(`The test run id is ${__ENV.K6_CLOUDRUN_TEST_RUN_ID}`);" + `
};`

		ts := makeTestState(t, script, []string{"--local-execution", "--log-output=stdout"})

		const pushRefID = "99999"
		ts.Env["K6_CLOUD_PUSH_REF_ID"] = pushRefID

		srv := getTestServer(t, map[string]http.Handler{
			"POST ^/v1/tests$": http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				require.Fail(t, "CreateTestRun must not be called when K6_CLOUD_PUSH_REF_ID is set")
			}),
		})
		t.Cleanup(srv.Close)
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		// MetricsPushURL is derived from the cloudapi.Client's BaseURL for the PushRefID path.

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)

		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, "output: cloud (https://app.k6.io/runs/"+pushRefID+")")
		assert.Contains(t, stdout, "The test run id is "+pushRefID)
	})
}

func makeTestState(tb testing.TB, script string, cliFlags []string) *GlobalTestState {
	if cliFlags == nil {
		cliFlags = []string{"-v", "--log-output=stdout"}
	}

	ts := NewGlobalTestState(tb)
	require.NoError(tb, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(script), 0o644))
	ts.CmdArgs = append(append([]string{"k6", "cloud", "run"}, cliFlags...), "test.js")
	ts.Env["K6_CLOUD_TOKEN"] = "foo"     // doesn't matter, we mock the cloud
	ts.Env["K6_CLOUD_STACK_ID"] = "1234" // doesn't matter, we mock the cloud

	return ts
}

// TestCloudRunLocalExecutionProvisioning_DefaultArchiveUpload verifies the full
// provisioning flow for Path A (k6 cloud run --local-execution) with archive upload:
//   - All provisioning endpoints are called
//   - Bearer auth on all provisioning/v6 calls
//   - X-Stack-Id header on v6 calls
//   - No Authorization header on S3 PUT
//   - Scoped test_run_token on metrics push
//   - No call to POST /v1/tests
func TestCloudRunLocalExecutionProvisioning_DefaultArchiveUpload(t *testing.T) {
	t.Parallel()

	var (
		mu              sync.Mutex
		capturedHeaders []http.Header
		endpointsCalled []string
	)

	srv := v6test.NewServer(t, v6test.Config{
		ArchiveUploadEnabled: true,
		InspectRequest: func(r *http.Request) {
			mu.Lock()
			capturedHeaders = append(capturedHeaders, r.Header.Clone())
			endpointsCalled = append(endpointsCalled, r.Method+" "+r.URL.Path)
			mu.Unlock()
		},
		ProgressCallback: func() *cloudapiv6.TestProgress {
			return &cloudapiv6.TestProgress{Status: cloudapiv6.StatusInitializing}
		},
	})

	ts := makeTestState(t, cloudLocalExecScript, []string{"--local-execution"})
	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
	ts.Env["K6_CLOUD_STACK_ID"] = "1"

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	t.Log("Endpoints called:", endpointsCalled)

	// Test completed successfully with the expected test run ID
	assert.Contains(t, stdout, "execution: local")
	assert.Contains(t, stdout, fmt.Sprintf("%d", v6test.DefaultTestRunID))

	mu.Lock()
	defer mu.Unlock()

	// At least the provisioning endpoints were called
	require.NotEmpty(t, endpointsCalled, "expected provisioning endpoints to be called")

	// Verify Bearer auth on every provisioning/v6 call
	for i, path := range endpointsCalled {
		h := capturedHeaders[i]
		// S3 PUT should NOT have Authorization header
		if path == "PUT /upload" {
			assert.Empty(t, h.Get("Authorization"),
				"S3 PUT at index %d should not have Authorization header, path=%s", i, path)
			continue
		}
		// Metrics push should use Bearer with scoped token
		if path == "POST /mock/metrics" {
			authHdr := h.Get("Authorization")
			assert.Equal(t, "Bearer mock-test-run-token", authHdr,
				"metrics push should use scoped test_run_token, path=%s", path)
			continue
		}
		// All other provisioning/v6 calls must use Bearer
		authHdr := h.Get("Authorization")
		assert.True(t, len(authHdr) > 0 && authHdr != "Token foo",
			"expected Bearer auth on path=%s, got %q", path, authHdr)
		assert.Contains(t, authHdr, "Bearer ",
			"expected Bearer auth scheme on path=%s, got %q", path, authHdr)
	}

	// Verify X-Stack-Id on v6 cloud API calls (only /cloud/v6/* endpoints use the SDK
	// which adds the X-Stack-Id header; the /provisioning/v1/* endpoints use a direct
	// HTTP request and do NOT include X-Stack-Id).
	for i, path := range endpointsCalled {
		if path == "POST /cloud/v6/projects/123456/load_tests" {
			h := capturedHeaders[i]
			assert.Equal(t, "1", h.Get("X-Stack-Id"),
				"expected X-Stack-Id: 1 on path=%s", path)
		}
	}

	// Verify no call to POST /v1/tests
	for _, path := range endpointsCalled {
		assert.NotContains(t, path, "/v1/tests",
			"expected no call to /v1/tests, but got %q", path)
	}
}

// TestCloudRunLocalExecutionProvisioning_NameConflict409 verifies that when the
// initial POST load_tests returns 409, the client falls back to GET and continues.
func TestCloudRunLocalExecutionProvisioning_NameConflict409(t *testing.T) {
	t.Parallel()

	ts := makeTestState(t, cloudLocalExecScript, []string{"--local-execution", "--no-archive-upload"})
	srv := v6test.NewServer(t, v6test.Config{
		ConflictOnCreateLoadTest: true,
		ProgressCallback: func() *cloudapiv6.TestProgress {
			return &cloudapiv6.TestProgress{Status: cloudapiv6.StatusInitializing}
		},
	})
	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
	ts.Env["K6_CLOUD_STACK_ID"] = "1"

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	stderr := ts.Stderr.String()
	t.Log("stdout:", stdout)
	t.Log("stderr:", stderr)

	// Test should complete successfully despite the initial 409
	assert.Contains(t, stdout, "execution: local")
	assert.Contains(t, stdout, fmt.Sprintf("%d", v6test.DefaultTestRunID))
}
