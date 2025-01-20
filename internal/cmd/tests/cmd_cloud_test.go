package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/cmd"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/fsext"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestK6Cloud(t *testing.T) {
	t.Parallel()
	runCloudTests(t, setupK6CloudCmd)
}

func setupK6CloudCmd(cliFlags []string) []string {
	return append([]string{"k6", "cloud"}, append(cliFlags, "test.js")...)
}

type setupCommandFunc func(cliFlags []string) []string

func runCloudTests(t *testing.T, setupCmd setupCommandFunc) {
	t.Run("TestCloudNotLoggedIn", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		ts.ExpectedExitCode = -1 // TODO: use a more specific exit code?
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `not logged in`)
	})

	t.Run("TestCloudLoggedInWithScriptToken", func(t *testing.T) {
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

		ts := getSimpleCloudTestState(t, []byte(script), setupCmd, nil, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.NotContains(t, stdout, `not logged in`)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Finished`)
	})

	t.Run("TestCloudExitOnRunning", func(t *testing.T) {
		t.Parallel()

		cs := func() cloudapi.TestProgressResponse {
			return cloudapi.TestProgressResponse{
				RunStatusText: "Running",
				RunStatus:     cloudapi.RunStatusRunning,
			}
		}

		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--exit-on-running", "--log-output=stdout"}, nil, cs)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Running`)
	})

	t.Run("TestCloudUploadOnly", func(t *testing.T) {
		t.Parallel()

		cs := func() cloudapi.TestProgressResponse {
			return cloudapi.TestProgressResponse{
				RunStatusText: "Archived",
				RunStatus:     cloudapi.RunStatusArchived,
			}
		}

		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--upload-only", "--log-output=stdout"}, nil, cs)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Archived`)
	})

	t.Run("TestCloudWithConfigOverride", func(t *testing.T) {
		t.Parallel()

		configOverride := http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			resp.WriteHeader(http.StatusOK)
			_, err := fmt.Fprint(resp, `{
			"reference_id": "123",
			"config": {
				"webAppURL": "https://bogus.url",
				"testRunDetails": "something from the cloud"
			},
			"logs": [
				{"level": "invalid", "message": "test debug message"},
				{"level": "warning", "message": "test warning"},
				{"level": "error", "message": "test error"}
			]
		}`)
			assert.NoError(t, err)
		})
		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, configOverride, nil)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: cloud")
		assert.Contains(t, stdout, "output: something from the cloud")
		assert.Contains(t, stdout, `level=debug msg="invalid message level 'invalid' for message 'test debug message'`)
		assert.Contains(t, stdout, `level=error msg="test debug message" source=grafana-k6-cloud`)
		assert.Contains(t, stdout, `level=warning msg="test warning" source=grafana-k6-cloud`)
		assert.Contains(t, stdout, `level=error msg="test error" source=grafana-k6-cloud`)
	})

	// TestCloudWithArchive tests that if k6 uses a static archive with the script inside that has cloud options like:
	//
	//	export let options = {
	//		ext: {
	//			loadimpact: {
	//				name: "my load test",
	//				projectID: 124,
	//				note: "lorem ipsum",
	//			},
	//		}
	//	};
	//
	// actually sends to the cloud the archive with the correct metadata (metadata.json), like:
	//
	//	"ext": {
	//		"loadimpact": {
	//	        "name": "my load test",
	//	        "note": "lorem ipsum",
	//	        "projectID": 124
	//	      }
	//	}
	t.Run("TestCloudWithArchive", func(t *testing.T) {
		t.Parallel()

		testRunID := 123
		ts := NewGlobalTestState(t)

		archiveUpload := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// check the archive
			file, _, err := req.FormFile("file")
			assert.NoError(t, err)
			assert.NotNil(t, file)

			// temporary write the archive for file system
			data, err := io.ReadAll(file)
			assert.NoError(t, err)

			tmpPath := filepath.Join(ts.Cwd, "archive_to_cloud.tar")
			require.NoError(t, fsext.WriteFile(ts.FS, tmpPath, data, 0o644))

			// check what inside
			require.NoError(t, testutils.Untar(t, ts.FS, tmpPath, "tmp/"))

			metadataRaw, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
			require.NoError(t, err)

			metadata := struct {
				Options struct {
					Cloud struct {
						Name      string `json:"name"`
						Note      string `json:"note"`
						ProjectID int    `json:"projectID"`
					} `json:"cloud"`
				} `json:"options"`
			}{}

			// then unpacked metadata should not contain any environment variables passed at the moment of archive creation
			require.NoError(t, json.Unmarshal(metadataRaw, &metadata))
			require.Equal(t, "my load test", metadata.Options.Cloud.Name)
			require.Equal(t, "lorem ipsum", metadata.Options.Cloud.Note)
			require.Equal(t, 124, metadata.Options.Cloud.ProjectID)

			// respond with the test run ID
			resp.WriteHeader(http.StatusOK)
			_, err = fmt.Fprintf(resp, `{"reference_id": "%d"}`, testRunID)
			assert.NoError(t, err)
		})

		srv := getMockCloud(t, testRunID, archiveUpload, nil)

		data, err := os.ReadFile(filepath.Join("testdata/archives", "archive_v0.46.0_with_loadimpact_option.tar")) //nolint:forbidigo // it's a test
		require.NoError(t, err)

		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), data, 0o644))

		ts.CmdArgs = []string{"k6", "cloud", "--verbose", "--log-output=stdout", "archive.tar"}
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false" // no mock for the logs yet
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_TOKEN"] = "foo" // doesn't matter, we mock the cloud

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.NotContains(t, stdout, `not logged in`)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `hello world from archive`)
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Finished`)
	})

	t.Run("TestCloudThresholdsHaveFailed", func(t *testing.T) {
		t.Parallel()

		progressCallback := func() cloudapi.TestProgressResponse {
			return cloudapi.TestProgressResponse{
				RunStatusText: "Finished",
				RunStatus:     cloudapi.RunStatusFinished,
				ResultStatus:  cloudapi.ResultStatusFailed,
				Progress:      1.0,
			}
		}
		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil, progressCallback)
		ts.ExpectedExitCode = int(exitcodes.ThresholdsHaveFailed)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `Thresholds have been crossed`)
	})

	t.Run("TestCloudAbortedThreshold", func(t *testing.T) {
		t.Parallel()

		progressCallback := func() cloudapi.TestProgressResponse {
			return cloudapi.TestProgressResponse{
				RunStatusText: "Finished",
				RunStatus:     cloudapi.RunStatusAbortedThreshold,
				ResultStatus:  cloudapi.ResultStatusFailed,
				Progress:      1.0,
			}
		}
		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil, progressCallback)
		ts.ExpectedExitCode = int(exitcodes.ThresholdsHaveFailed)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `Thresholds have been crossed`)
	})
}

func cloudTestStartSimple(tb testing.TB, testRunID int) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		resp.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(resp, `{"reference_id": "%d"}`, testRunID)
		assert.NoError(tb, err)
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
		RunStatus:     cloudapi.RunStatusFinished,
		ResultStatus:  cloudapi.ResultStatusPassed,
		Progress:      1,
	}

	srv := getTestServer(t, map[string]http.Handler{
		"POST ^/v1/archive-upload$": archiveUpload,
		testProgressURL: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
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

func getSimpleCloudTestState(t *testing.T, script []byte, setupCmd setupCommandFunc, cliFlags []string, archiveUpload http.Handler, progressCallback func() cloudapi.TestProgressResponse) *GlobalTestState {
	if script == nil {
		script = []byte(`export default function() {}`)
	}

	if cliFlags == nil {
		cliFlags = []string{"--verbose", "--log-output=stdout"}
	}

	srv := getMockCloud(t, 123, archiveUpload, progressCallback)

	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), script, 0o644))
	ts.CmdArgs = setupCmd(cliFlags)
	ts.Env["K6_SHOW_CLOUD_LOGS"] = "false" // no mock for the logs yet
	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_TOKEN"] = "foo" // doesn't matter, we mock the cloud

	return ts
}
