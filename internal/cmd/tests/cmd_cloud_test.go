package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/errext/exitcodes"
	v6cloudapi "go.k6.io/k6/internal/cloudapi/v6"
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
	t.Run("TestCloudUserNotAuthenticated", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, setupCmd, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		ts.ExpectedExitCode = -1 // TODO: use a more specific exit code?
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `must first authenticate`)
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

		ts := getSimpleRemoteCloudTestState(t, []byte(script), setupCmd, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.NotContains(t, stdout, `not logged in`)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Completed`)
	})

	t.Run("TestCloudExitOnRunning", func(t *testing.T) {
		t.Parallel()

		cs := func() v6cloudapi.TestRunProgress {
			return v6cloudapi.TestRunProgress{
				Status:            v6cloudapi.StatusRunning,
				ExecutionDuration: 10,
				EstimatedDuration: 20,
			}
		}

		ts := getSimpleRemoteCloudTestState(t, nil, setupCmd, []string{"--exit-on-running", "--log-output=stdout"}, cs)
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

		ts := getSimpleCloudTestState(t, setupCmd, []string{"--upload-only", "--log-output=stdout"}, cs)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Archived`)
	})

	t.Run("TestCloudRemoteValidateUsesV6Endpoint", func(t *testing.T) {
		t.Parallel()

		var validated atomic.Bool
		routes := newRemoteCloudRoutes(t, nil)
		routes["POST ^/cloud/v6/validate_options$"] = http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			validated.Store(true)
			assert.Equal(t, "123", req.Header.Get("X-Stack-Id"))

			var body struct {
				Options map[string]any `json:"options"`
			}
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			assert.EqualValues(t, 10, body.Options["vus"])
			assert.Equal(t, "10s", body.Options["duration"])

			resp.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(resp, `{}`)
			assert.NoError(t, err)
		})
		srv := getTestServer(t, routes)
		t.Cleanup(srv.Close)

		ts := NewGlobalTestState(t)
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(`
			export const options = { vus: 10, duration: "10s" };
			export default function() {}
		`), 0o644))
		ts.CmdArgs = setupCmd([]string{"--verbose", "--log-output=stdout"})
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_STACK_ID"] = "123"
		ts.Env["K6_CLOUD_PROJECT_ID"] = "124"
		ts.Env["K6_CLOUD_TOKEN"] = "foo"

		cmd.ExecuteWithGlobalState(ts.GlobalState)
		assert.True(t, validated.Load())
	})
	// TestCloudWithArchive tests that if k6 uses a static archive with the script inside that has cloud options like:
	//
	//	export let options = {
	//		cloud: {
	//			name: "my load test",
	//			projectID: 124,
	//			note: "lorem ipsum",
	//		}
	//	};
	//
	// actually sends to the cloud the archive with the correct metadata (metadata.json), like:
	//
	//	"clouad": {
	//	    "name": "my load test",
	//	    "note": "lorem ipsum",
	//	    "projectID": 124
	//	}
	t.Run("TestCloudWithArchive", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		createHandler := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			file, _, err := req.FormFile("script")
			assert.NoError(t, err)
			assert.NotNil(t, file)

			data, err := io.ReadAll(file)
			assert.NoError(t, err)

			tmpPath := filepath.Join(ts.Cwd, "archive_to_cloud.tar")
			require.NoError(t, fsext.WriteFile(ts.FS, tmpPath, data, 0o644))
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
			require.NoError(t, json.Unmarshal(metadataRaw, &metadata))
			require.Equal(t, "my load test", metadata.Options.Cloud.Name)
			require.Equal(t, "lorem ipsum", metadata.Options.Cloud.Note)
			require.Equal(t, 124, metadata.Options.Cloud.ProjectID)

			resp.Header().Set("Content-Type", "application/json")
			resp.WriteHeader(http.StatusCreated)
			_, err = fmt.Fprint(resp, newLoadTestJSON(t, "my load test"))
			assert.NoError(t, err)
		})

		routes := newRemoteCloudRoutes(t, nil)
		routes[`POST ^/cloud/v6/projects/\d+/load_tests$`] = createHandler
		srv := getTestServer(t, routes)
		t.Cleanup(srv.Close)

		//nolint:forbidigo // it's a test
		data, err := os.ReadFile(
			filepath.Join("testdata/archives", "archive_v1.0.0_with_cloud_option.tar"),
		)
		require.NoError(t, err)
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), data, 0o644))

		ts.CmdArgs = []string{"k6", "cloud", "--verbose", "--log-output=stdout", "archive.tar"}
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false" // no mock for the logs yet
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_STACK_ID"] = "123"
		ts.Env["K6_CLOUD_PROJECT_ID"] = "124"
		ts.Env["K6_CLOUD_TOKEN"] = "foo" // doesn't matter, we mock the cloud

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.NotContains(t, stdout, `not logged in`)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `hello world from archive`)
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Completed`)
	})

	t.Run("TestCloudAbortUsesV6Endpoint", func(t *testing.T) {
		t.Parallel()

		var aborted atomic.Bool
		routes := newRemoteCloudRoutes(t, func() v6cloudapi.TestRunProgress {
			if aborted.Load() {
				return v6cloudapi.TestRunProgress{
					Status:            v6cloudapi.StatusCompleted,
					EstimatedDuration: 10,
					ExecutionDuration: 10,
				}
			}

			return v6cloudapi.TestRunProgress{
				Status:            v6cloudapi.StatusRunning,
				EstimatedDuration: 10,
				ExecutionDuration: 1,
			}
		})
		routes[`POST ^/cloud/v6/test_runs/\d+/abort$`] = http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			aborted.Store(true)
			resp.WriteHeader(http.StatusNoContent)
		})
		srv := getTestServer(t, routes)
		t.Cleanup(srv.Close)

		ts := NewGlobalTestState(t)
		require.NoError(t, fsext.WriteFile(
			ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(`export default function() {}`), 0o644,
		))
		ts.CmdArgs = setupCmd([]string{"--verbose", "--log-output=stdout"})
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_STACK_ID"] = "123"
		ts.Env["K6_CLOUD_PROJECT_ID"] = "124"
		ts.Env["K6_CLOUD_TOKEN"] = "foo"

		asyncWaitForStdoutAndStopTestWithInterruptSignal(t, ts, 20, 200*time.Millisecond, "execution: cloud")
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.True(t, aborted.Load(), "remote abort must use the v6 endpoint")
	})

	t.Run("TestCloudUpdatesExistingRemoteTest", func(t *testing.T) {
		t.Parallel()

		var updateCalls atomic.Int32

		routes := map[string]http.Handler{
			"POST ^/cloud/v6/validate_options$": http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(resp, `{}`)
				assert.NoError(t, err)
			}),
			`POST ^/cloud/v6/projects/\d+/load_tests$`: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "application/json")
				resp.WriteHeader(http.StatusConflict)
				_, err := fmt.Fprint(resp, `{"error":{"code":"conflict","message":"load test already exists","details":[]}}`)
				assert.NoError(t, err)
			}),
			`GET ^/cloud/v6/load_tests.*$`: http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				assert.Equal(t, "test.js", req.URL.Query().Get("name"))

				list := k6cloud.NewLoadTestListResponse([]k6cloud.LoadTestApiModel{
					*k6cloud.NewLoadTestApiModel(
						455,
						124,
						"test.js-copy",
						*k6cloud.NewNullableInt32(nil),
						time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					),
					*k6cloud.NewLoadTestApiModel(
						456,
						124,
						"test.js",
						*k6cloud.NewNullableInt32(nil),
						time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					),
				})

				resp.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(resp, mustJSON(t, list))
				assert.NoError(t, err)
			}),
			`PUT ^/cloud/v6/load_tests/456/script$`: http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				updateCalls.Add(1)

				data, err := io.ReadAll(req.Body)
				assert.NoError(t, err)
				assert.NotEmpty(t, data)

				resp.WriteHeader(http.StatusNoContent)
			}),
			`POST ^/cloud/v6/load_tests/456/start$`: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(resp, newStartLoadTestResponseJSON(t, 123))
				assert.NoError(t, err)
			}),
			`GET ^/cloud/v6/test_runs/\d+$`: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(resp, newTestRunJSON(t, 123, v6cloudapi.TestRunProgress{
					Status: v6cloudapi.StatusCompleted,
					Result: v6cloudapi.ResultPassed,
				}))
				assert.NoError(t, err)
			}),
		}
		srv := getTestServer(t, routes)
		t.Cleanup(srv.Close)

		ts := NewGlobalTestState(t)
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(`export default function() {}`), 0o644))
		ts.CmdArgs = setupCmd([]string{"--verbose", "--log-output=stdout"})
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_STACK_ID"] = "123"
		ts.Env["K6_CLOUD_PROJECT_ID"] = "124"
		ts.Env["K6_CLOUD_TOKEN"] = "foo"

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.EqualValues(t, 1, updateCalls.Load())
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Completed`)
	})

	t.Run("TestCloudThresholdsHaveFailed", func(t *testing.T) {
		t.Parallel()

		progressCallback := func() v6cloudapi.TestRunProgress {
			return v6cloudapi.TestRunProgress{
				Status: v6cloudapi.StatusCompleted,
				Result: v6cloudapi.ResultFailed,
			}
		}
		ts := getSimpleRemoteCloudTestState(t, nil, setupCmd, nil, progressCallback)
		ts.ExpectedExitCode = int(exitcodes.ThresholdsHaveFailed)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `Thresholds have been crossed`)
	})

	t.Run("TestCloudAbortedThreshold", func(t *testing.T) {
		t.Parallel()

		progressCallback := func() v6cloudapi.TestRunProgress {
			return v6cloudapi.TestRunProgress{
				Status: v6cloudapi.StatusAbortedThreshold,
				Result: v6cloudapi.ResultFailed,
			}
		}
		ts := getSimpleRemoteCloudTestState(t, nil, setupCmd, nil, progressCallback)
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

func getSimpleCloudTestState(t *testing.T, setupCmd setupCommandFunc, cliFlags []string, progressCallback func() cloudapi.TestProgressResponse) *GlobalTestState {
	if cliFlags == nil {
		cliFlags = []string{"--verbose", "--log-output=stdout"}
	}

	srv := getMockCloud(t, 123, nil, progressCallback)

	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(`export default function() {}`), 0o644))
	ts.CmdArgs = setupCmd(cliFlags)
	ts.Env["K6_SHOW_CLOUD_LOGS"] = "false" // no mock for the logs yet
	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_TOKEN"] = "foo" // doesn't matter, we mock the cloud

	return ts
}

func getSimpleRemoteCloudTestState(t *testing.T, script []byte, setupCmd setupCommandFunc, cliFlags []string, progressCallback func() v6cloudapi.TestRunProgress) *GlobalTestState {
	if script == nil {
		script = []byte(`export default function() {}`)
	}

	if cliFlags == nil {
		cliFlags = []string{"--verbose", "--log-output=stdout"}
	}

	srv := getTestServer(t, newRemoteCloudRoutes(t, progressCallback))
	t.Cleanup(srv.Close)

	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), script, 0o644))
	ts.CmdArgs = setupCmd(cliFlags)
	ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_STACK_ID"] = "123"
	ts.Env["K6_CLOUD_PROJECT_ID"] = "124"
	ts.Env["K6_CLOUD_TOKEN"] = "foo"

	return ts
}

func newRemoteCloudRoutes(t testing.TB, progressCallback func() v6cloudapi.TestRunProgress) map[string]http.Handler {
	t.Helper()

	defaultProgress := v6cloudapi.TestRunProgress{
		Status:            v6cloudapi.StatusCompleted,
		Result:            v6cloudapi.ResultPassed,
		EstimatedDuration: 10,
		ExecutionDuration: 10,
	}

	return map[string]http.Handler{
		"POST ^/cloud/v6/validate_options$": http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "123", req.Header.Get("X-Stack-Id"))
			resp.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(resp, `{}`)
			assert.NoError(t, err)
		}),
		`POST ^/cloud/v6/projects/\d+/load_tests$`: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			resp.Header().Set("Content-Type", "application/json")
			resp.WriteHeader(http.StatusCreated)
			_, err := fmt.Fprint(resp, newLoadTestJSON(t, "test-name"))
			assert.NoError(t, err)
		}),
		`POST ^/cloud/v6/load_tests/\d+/start$`: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			resp.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(resp, newStartLoadTestResponseJSON(t, 123))
			assert.NoError(t, err)
		}),
		`GET ^/cloud/v6/test_runs/\d+$`: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			testProgress := defaultProgress
			if progressCallback != nil {
				testProgress = progressCallback()
			}

			resp.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(resp, newTestRunJSON(t, 123, testProgress))
			assert.NoError(t, err)
		}),
	}
}

func newStartLoadTestResponseJSON(tb testing.TB, testRunID int) string {
	tb.Helper()

	statusDetails := *k6cloud.NewStatusApiModel(v6cloudapi.StatusCreated, time.Unix(0, 0).UTC())
	res := k6cloud.NewStartLoadTestResponse(
		int32(testRunID),
		456,
		124,
		*k6cloud.NewNullableString(nil),
		time.Unix(0, 0).UTC(),
		*k6cloud.NewNullableTime(nil),
		"",
		*k6cloud.NewNullableTime(nil),
		*k6cloud.NewNullableTestCostApiModel(nil),
		v6cloudapi.StatusRunning,
		statusDetails,
		[]k6cloud.StatusApiModel{statusDetails},
		[]k6cloud.DistributionZoneApiModel{},
		*k6cloud.NewNullableString(nil),
		map[string]any{},
		map[string]any{},
		map[string]string{},
		map[string]string{},
		*k6cloud.NewNullableInt32(nil),
		*k6cloud.NewNullableInt32(nil),
		*k6cloud.NewNullableInt32(nil),
		0,
		fmt.Sprintf("https://app.k6.io/runs/%d", testRunID),
	)

	return mustJSON(tb, res)
}

func newTestRunJSON(tb testing.TB, testRunID int, progress v6cloudapi.TestRunProgress) string {
	tb.Helper()

	statusDetails := *k6cloud.NewStatusApiModel(progress.Status, time.Unix(0, 0).UTC())
	var result *string
	if progress.Result != "" {
		result = &progress.Result
	}
	var estimatedDuration *int32
	if progress.EstimatedDuration > 0 {
		estimatedDuration = &progress.EstimatedDuration
	}

	res := k6cloud.NewTestRunApiModel(
		int32(testRunID),
		456,
		124,
		*k6cloud.NewNullableString(nil),
		time.Unix(0, 0).UTC(),
		*k6cloud.NewNullableTime(nil),
		"",
		*k6cloud.NewNullableTime(nil),
		*k6cloud.NewNullableTestRunApiModelCost(nil),
		progress.Status,
		statusDetails,
		[]k6cloud.StatusApiModel{statusDetails},
		[]k6cloud.DistributionZoneApiModel{},
		*k6cloud.NewNullableString(result),
		map[string]any{},
		map[string]any{},
		map[string]string{},
		map[string]string{},
		*k6cloud.NewNullableInt32(nil),
		*k6cloud.NewNullableInt32(nil),
		*k6cloud.NewNullableInt32(estimatedDuration),
		progress.ExecutionDuration,
	)

	return mustJSON(tb, res)
}

func newLoadTestJSON(tb testing.TB, name string) string {
	tb.Helper()

	res := k6cloud.NewLoadTestApiModel(
		456,
		124,
		name,
		*k6cloud.NewNullableInt32(nil),
		time.Unix(0, 0).UTC(),
		time.Unix(0, 0).UTC(),
	)

	return mustJSON(tb, res)
}

func mustJSON(tb testing.TB, v any) string {
	tb.Helper()

	data, err := json.Marshal(v)
	require.NoError(tb, err)

	return string(data)
}
