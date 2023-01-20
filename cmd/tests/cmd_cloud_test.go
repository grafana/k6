package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd"
	"go.k6.io/k6/lib"
)

func cloudTestStartSimple(t *testing.T, testRunID int) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(resp, `{"reference_id": "%d"}`, testRunID)
		assert.NoError(t, err)
	})
}

func getMockCloud(
	t *testing.T, testRunID int,
	archiveUpload http.Handler, progressCallback func() cloudapi.TestProgressResponse,
) *httptest.Server {
	if archiveUpload == nil {
		archiveUpload = cloudTestStartSimple(t, testRunID)
	}
	testProgressURL := fmt.Sprintf("GET ^/v1/test-progress/%d$", testRunID)
	defaultProgress := cloudapi.TestProgressResponse{
		RunStatusText: "Finished",
		RunStatus:     lib.RunStatusFinished,
		ResultStatus:  cloudapi.ResultStatusPassed,
		Progress:      1,
	}

	srv := getTestServer(t, map[string]http.Handler{
		"POST ^/v1/archive-upload$": archiveUpload,
		testProgressURL: http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			testProgress := defaultProgress
			if progressCallback != nil {
				testProgress = progressCallback()
			}
			respBody, err := json.Marshal(testProgress)
			assert.NoError(t, err)
			_, err = fmt.Fprint(resp, string(respBody))
			assert.NoError(t, err)
		}),
	})

	t.Cleanup(srv.Close)

	return srv
}

func getSimpleCloudTestState(
	t *testing.T, script []byte, cliFlags []string,
	progressCallback func() cloudapi.TestProgressResponse,
) *GlobalTestState {
	if script == nil {
		script = []byte(`export default function() {}`)
	}

	if cliFlags == nil {
		cliFlags = []string{"--verbose", "--log-output=stdout"}
	}

	srv := getMockCloud(t, 123, nil, progressCallback)

	ts := NewGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), script, 0o644))
	ts.CmdArgs = append(append([]string{"k6", "cloud"}, cliFlags...), "test.js")
	ts.Env["K6_SHOW_CLOUD_LOGS"] = "false" // no mock for the logs yet
	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_TOKEN"] = "foo" // doesn't matter, we mock the cloud

	return ts
}

func TestCloudNotLoggedIn(t *testing.T) {
	t.Parallel()

	ts := getSimpleCloudTestState(t, nil, nil, nil)
	delete(ts.Env, "K6_CLOUD_TOKEN")
	ts.ExpectedExitCode = -1 // TODO: use a more specific exit code?
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, `Not logged in`)
}

func TestCloudLoggedInWithScriptToken(t *testing.T) {
	t.Parallel()

	script := `
		export let options = {
			ext: {
				loadimpact: {
					token: "asdf",
					name: "my load test",
					projectID: 124,
					note: 124,
				},
			}
		};
		export default function() {};
	`

	ts := getSimpleCloudTestState(t, []byte(script), nil, nil)
	delete(ts.Env, "K6_CLOUD_TOKEN")
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotContains(t, stdout, `Not logged in`)
	assert.Contains(t, stdout, `execution: cloud`)
	assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
	assert.Contains(t, stdout, `test status: Finished`)
}

func TestCloudExitOnRunning(t *testing.T) {
	t.Parallel()

	cs := func() cloudapi.TestProgressResponse {
		return cloudapi.TestProgressResponse{
			RunStatusText: "Running",
			RunStatus:     lib.RunStatusRunning,
		}
	}

	ts := getSimpleCloudTestState(t, nil, []string{"--exit-on-running", "--log-output=stdout"}, cs)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.Contains(t, stdout, `execution: cloud`)
	assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
	assert.Contains(t, stdout, `test status: Running`)
}
