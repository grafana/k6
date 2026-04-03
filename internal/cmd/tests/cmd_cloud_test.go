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
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/errext/exitcodes"
	v6cloudapi "go.k6.io/k6/internal/cloudapi/v6"
	"go.k6.io/k6/internal/cloudapi/v6/v6testing"
	"go.k6.io/k6/internal/cmd"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/fsext"
)

func TestK6Cloud(t *testing.T) {
	t.Parallel()
	runCloudTests(t, setupK6CloudCmd)
}

func setupK6CloudCmd(cliFlags []string) []string {
	return append([]string{"k6", "cloud"}, append(cliFlags, "test.js")...)
}

type setupCommandFunc func(cliFlags []string) []string

const testStackURL = "https://app.k6.io"

func runCloudTests(t *testing.T, setupCmd setupCommandFunc) {
	t.Run("TestCloudUserNotAuthenticated", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil, nil)
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

		ts := getSimpleCloudTestState(t, []byte(script), setupCmd, nil, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.NotContains(t, stdout, `not logged in`)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, "output: "+testStackURL+"/a/k6-app/runs/123")
		assert.Contains(t, stdout, `test status: Completed`)
	})

	t.Run("TestCloudExitOnRunning", func(t *testing.T) {
		t.Parallel()

		cs := func() v6cloudapi.TestRunProgress {
			return v6cloudapi.TestRunProgress{
				Status:            v6cloudapi.StatusRunning,
				EstimatedDuration: 10,
				ExecutionDuration: 1,
			}
		}

		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--exit-on-running", "--log-output=stdout"}, nil, cs)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, "output: "+testStackURL+"/a/k6-app/runs/123")
		assert.Contains(t, stdout, `test status: Running`)
	})

	t.Run("TestCloudUploadOnly", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--upload-only", "--log-output=stdout"}, nil, nil)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, "output: "+testStackURL+"/a/k6-app/tests/456")
		assert.Contains(t, stdout, `test status: Uploaded`)
	})

	t.Run("TestCloudUploadOnlyTrailingSlash", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--upload-only", "--log-output=stdout"}, nil, nil)
		ts.Env["K6_CLOUD_STACK_URL"] = testStackURL + "/"
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "output: "+testStackURL+"/a/k6-app/tests/456")
		assert.NotContains(t, stdout, "//a/k6-app")
	})

	t.Run("TestCloudUploadOnlyNoStackURL", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--upload-only", "--log-output=stdout"}, nil, nil)
		delete(ts.Env, "K6_CLOUD_STACK_URL")
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `test status: Uploaded`)
		assert.Contains(t, stdout, "output: -")
		// Without StackURL, no URL should be shown — neither a broken
		// relative path nor a run URL (upload-only has no test run).
		assert.NotContains(t, stdout, `output: /a/k6-app/tests/`)
		assert.NotContains(t, stdout, `/runs/`)
	})

	t.Run("TestCloudStartResponseURL", func(t *testing.T) {
		t.Parallel()

		// v6 start response includes test_run_details_page_url which the
		// CLI uses directly as the output URL (no ConfigOverride like v1).
		testRunURL := "https://custom.cloud.url/runs/123"

		startHandler := http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			writeJSON(resp, http.StatusOK,
				v6testing.TestRunJSON(t, 123, v6cloudapi.StatusRunning, nil, testRunURL))
		})

		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, startHandler, nil)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: cloud")
		assert.Contains(t, stdout, "output: "+testRunURL)
	})

	// TestCloudLogStreaming proves that the v6 numeric run ID works as
	// refID for the log streaming WebSocket endpoint. The test enables
	// --show-logs and provides a WebSocket mock that sends one log entry,
	// then verifies the message appears in stdout.
	t.Run("TestCloudLogStreaming", func(t *testing.T) {
		t.Parallel()

		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		logSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the refID is the v6 numeric test run ID
			assert.Contains(t, r.URL.RawQuery, `test_run_id="123"`)

			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("websocket upgrade error: %v", err)
				return
			}
			defer func() { _ = conn.Close() }()

			msg := `{"streams":[{"stream":{"level":"info"},"values":[["1704067200000000000","log entry from cloud"]]}]}`
			_ = conn.WriteMessage(websocket.TextMessage, []byte(msg))

			// Keep the connection open until the client disconnects.
			for {
				if _, _, rerr := conn.ReadMessage(); rerr != nil {
					break
				}
			}
		}))
		defer logSrv.Close()

		// Replace https:// with ws:// for the WebSocket URL
		wsURL := "ws" + logSrv.URL[len("http"):]

		ts := getSimpleCloudTestState(t, nil, setupCmd, []string{"--show-logs", "--verbose", "--log-output=stdout"}, nil, nil)
		ts.Env["K6_CLOUD_LOGS_TAIL_URL"] = wsURL + "/api/v1/tail"

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `test status: Completed`)
		assert.Contains(t, stdout, `log entry from cloud`)
	})

	// TestCloudWithArchive verifies that when running from a pre-built
	// archive, the uploaded archive metadata reflects the K6_CLOUD_PROJECT_ID
	// env var (456) rather than the script-embedded projectID (124).
	t.Run("TestCloudWithArchive", func(t *testing.T) {
		t.Parallel()

		testRunID := 123
		ts := NewGlobalTestState(t)

		archiveUpload := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// check the archive
			file, _, err := req.FormFile("script")
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
			// projectID is overridden by K6_CLOUD_PROJECT_ID env var (456)
			require.Equal(t, 456, metadata.Options.Cloud.ProjectID)

			// respond with the load test
			writeJSON(resp, http.StatusCreated, newLoadTestJSON())
		})

		srv := getMockCloud(t, mockCloudOpts{testRunID: testRunID, createHandler: archiveUpload})

		data, err := os.ReadFile(filepath.Join("testdata/archives", "archive_v1.0.0_with_cloud_option.tar")) //nolint:forbidigo // it's a test
		require.NoError(t, err)

		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), data, 0o644))

		ts.CmdArgs = []string{"k6", "cloud", "--verbose", "--log-output=stdout", "archive.tar"}
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false" // no mock for the logs yet
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
		ts.Env["K6_CLOUD_TOKEN"] = "foo" // doesn't matter, we mock the cloud
		ts.Env["K6_CLOUD_STACK_ID"] = "123"
		ts.Env["K6_CLOUD_PROJECT_ID"] = "456"
		ts.Env["K6_CLOUD_STACK_URL"] = testStackURL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.NotContains(t, stdout, `not logged in`)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `hello world from archive`)
		assert.Contains(t, stdout, "output: "+testStackURL+"/a/k6-app/runs/123")
		assert.Contains(t, stdout, `test status: Completed`)
	})

	// TestCloudWithArchiveScriptProjectID verifies that when no
	// K6_CLOUD_PROJECT_ID env var is set, the script-embedded projectID
	// (124) is preserved in the uploaded archive metadata.
	t.Run("TestCloudWithArchiveScriptProjectID", func(t *testing.T) {
		t.Parallel()

		testRunID := 123
		ts := NewGlobalTestState(t)

		archiveUpload := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
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
						ProjectID int `json:"projectID"`
					} `json:"cloud"`
				} `json:"options"`
			}{}

			require.NoError(t, json.Unmarshal(metadataRaw, &metadata))
			// No K6_CLOUD_PROJECT_ID set, so the script-embedded value (124) survives.
			require.Equal(t, 124, metadata.Options.Cloud.ProjectID)

			writeJSON(resp, http.StatusCreated, newLoadTestJSON())
		})

		srv := getMockCloud(t, mockCloudOpts{testRunID: testRunID, createHandler: archiveUpload})

		data, err := os.ReadFile(filepath.Join("testdata/archives", "archive_v1.0.0_with_cloud_option.tar")) //nolint:forbidigo // it's a test
		require.NoError(t, err)
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), data, 0o644))

		ts.CmdArgs = []string{"k6", "cloud", "--verbose", "--log-output=stdout", "archive.tar"}
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
		ts.Env["K6_CLOUD_TOKEN"] = "foo"
		ts.Env["K6_CLOUD_STACK_ID"] = "123"
		// K6_CLOUD_PROJECT_ID intentionally NOT set
		ts.Env["K6_CLOUD_STACK_URL"] = testStackURL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `test status: Completed`)
	})

	t.Run("TestCloudThresholdsHaveFailed", func(t *testing.T) {
		t.Parallel()

		progressCallback := func() v6cloudapi.TestRunProgress {
			return v6cloudapi.TestRunProgress{
				Status:            v6cloudapi.StatusCompleted,
				Result:            v6cloudapi.ResultFailed,
				EstimatedDuration: 10,
				ExecutionDuration: 10,
			}
		}
		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil, progressCallback)
		ts.ExpectedExitCode = int(exitcodes.ThresholdsHaveFailed)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `Thresholds have been crossed`)
	})

	t.Run("TestCloudAbortedFailed", func(t *testing.T) {
		t.Parallel()

		// Per the v6 spec, result "failed" always means thresholds breached,
		// even when the test was aborted (e.g. aborted due to threshold).
		progressCallback := func() v6cloudapi.TestRunProgress {
			return v6cloudapi.TestRunProgress{
				Status:            v6cloudapi.StatusAborted,
				Result:            v6cloudapi.ResultFailed,
				EstimatedDuration: 10,
				ExecutionDuration: 10,
			}
		}
		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil, progressCallback)
		ts.ExpectedExitCode = int(exitcodes.ThresholdsHaveFailed)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `Thresholds have been crossed`)
		assert.Contains(t, stdout, `test status: Aborted`)
	})

	t.Run("TestCloudResultError", func(t *testing.T) {
		t.Parallel()

		progressCallback := func() v6cloudapi.TestRunProgress {
			return v6cloudapi.TestRunProgress{
				Status:            v6cloudapi.StatusCompleted,
				Result:            v6cloudapi.ResultError,
				EstimatedDuration: 10,
				ExecutionDuration: 10,
			}
		}
		ts := getSimpleCloudTestState(t, nil, setupCmd, nil, nil, progressCallback)
		ts.ExpectedExitCode = int(exitcodes.CloudTestRunFailed)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `The test has failed`)
	})

	// TestCloudUploadOnlyNeverAborts proves that upload-only never calls
	// the test run abort endpoint. An earlier version shared testRunID and
	// signal handling across both paths, so SIGINT during upload-only could
	// send a load test ID to POST /test_runs/{id}/abort.
	t.Run("TestCloudUploadOnlyNeverAborts", func(t *testing.T) {
		t.Parallel()

		var aborted atomic.Bool
		abortHandler := http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			aborted.Store(true)
			resp.WriteHeader(http.StatusNoContent)
		})

		srv := getTestServer(t, map[string]http.Handler{
			"POST ^/cloud/v6/validate_options$":        http.HandlerFunc(v6ValidateOptionsHandler),
			`POST ^/cloud/v6/projects/\d+/load_tests$`: cloudTestCreateSimple(t),
			`PUT ^/cloud/v6/load_tests/\d+/script$`:    http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) { resp.WriteHeader(http.StatusNoContent) }),
			`POST ^/cloud/v6/load_tests/\d+/start$`:    http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("start should not be called in upload-only") }),
			`POST ^/cloud/v6/test_runs/\d+/abort$`:     abortHandler,
			`GET ^/cloud/v6/projects/\d+/load_tests`: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
				writeJSON(resp, http.StatusOK, fmt.Sprintf(`{"value": [%s]}`, newLoadTestJSON()))
			}),
		})
		t.Cleanup(srv.Close)

		ts := NewGlobalTestState(t)
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(`export default function() {}`), 0o644))
		ts.CmdArgs = setupCmd([]string{"--upload-only", "--verbose", "--log-output=stdout"})
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
		ts.Env["K6_CLOUD_TOKEN"] = "foo"
		ts.Env["K6_CLOUD_STACK_ID"] = "123"
		ts.Env["K6_CLOUD_PROJECT_ID"] = "456"
		ts.Env["K6_CLOUD_STACK_URL"] = testStackURL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `test status: Uploaded`)
		assert.False(t, aborted.Load(), "abort endpoint must not be called during upload-only")
	})

	t.Run("TestCloudGracefulStopWaitsForTerminal", func(t *testing.T) {
		t.Parallel()

		var aborted atomic.Bool
		progressCallback := func() v6cloudapi.TestRunProgress {
			if aborted.Load() {
				return v6cloudapi.TestRunProgress{
					Status:            v6cloudapi.StatusAborted,
					EstimatedDuration: 60,
					ExecutionDuration: 30,
				}
			}
			return v6cloudapi.TestRunProgress{
				Status:            v6cloudapi.StatusRunning,
				EstimatedDuration: 60,
				ExecutionDuration: 5,
			}
		}

		testRunID := 123
		defaultWebAppURL := fmt.Sprintf("%s/a/k6-app/runs/%d", testStackURL, testRunID)
		defaultProgress := v6cloudapi.TestRunProgress{
			Status:            v6cloudapi.StatusRunning,
			EstimatedDuration: 60,
			ExecutionDuration: 5,
		}

		abortHandler := http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			aborted.Store(true)
			resp.WriteHeader(http.StatusNoContent)
		})

		srv := getTestServer(t, map[string]http.Handler{
			"POST ^/cloud/v6/validate_options$":                           http.HandlerFunc(v6ValidateOptionsHandler),
			`POST ^/cloud/v6/projects/\d+/load_tests$`:                    cloudTestCreateSimple(t),
			`PUT ^/cloud/v6/load_tests/\d+/script$`:                       http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) { resp.WriteHeader(http.StatusNoContent) }),
			`POST ^/cloud/v6/load_tests/\d+/start$`:                       cloudTestStartSimple(t, testRunID, defaultWebAppURL),
			fmt.Sprintf("GET ^/cloud/v6/test_runs/%d$", testRunID):        v6ProgressHandler(t, testRunID, defaultWebAppURL, defaultProgress, progressCallback),
			fmt.Sprintf("POST ^/cloud/v6/test_runs/%d/abort$", testRunID): abortHandler,
			`GET ^/cloud/v6/projects/\d+/load_tests`: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
				writeJSON(resp, http.StatusOK, fmt.Sprintf(`{"value": [%s]}`, newLoadTestJSON()))
			}),
		})
		t.Cleanup(srv.Close)

		ts := NewGlobalTestState(t)
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(`export default function() {}`), 0o644))
		ts.CmdArgs = setupCmd([]string{"--verbose", "--log-output=stdout"})
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
		ts.Env["K6_CLOUD_TOKEN"] = "foo"
		ts.Env["K6_CLOUD_STACK_ID"] = "123"
		ts.Env["K6_CLOUD_PROJECT_ID"] = "456"
		ts.Env["K6_CLOUD_STACK_URL"] = testStackURL

		sendSignal := injectMockSignalNotifier(ts)
		asyncWaitForStdoutAndRun(t, ts, 20, 500*time.Millisecond, "Trapping interrupt signals", func() {
			t.Log("signal trap is set, sending SIGINT...")
			sendSignal <- syscall.SIGINT
			<-sendSignal
		})

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `test status: Aborted`)
		assert.True(t, aborted.Load(), "abort endpoint should have been called")
	})
}

var testEpoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) //nolint:gochecknoglobals

// marshalJSON marshals v to JSON; panics on error (test helper).
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// newLoadTestJSON builds a LoadTestApiModel JSON response string.
// Using the generated model ensures fixtures break at compile time
// if the spec adds or renames required fields.
func newLoadTestJSON() string {
	return marshalJSON(k6cloud.NewLoadTestApiModel(
		456, 789, "test", *k6cloud.NewNullableInt32(nil),
		testEpoch, testEpoch,
	))
}

func cloudTestStartSimpleV1(tb testing.TB, testRunID int) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		resp.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(resp, `{"reference_id": "%d"}`, testRunID)
		assert.NoError(tb, err)
	})
}

func cloudTestCreateSimple(_ testing.TB) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		writeJSON(resp, http.StatusCreated, newLoadTestJSON())
	})
}

func cloudTestStartSimple(t testing.TB, testRunID int, webAppURL string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		writeJSON(resp, http.StatusOK,
			v6testing.TestRunJSON(t, int32(testRunID), "running", nil, webAppURL))
	})
}

// writeJSON sets Content-Type and writes a JSON body to the response.
func writeJSON(resp http.ResponseWriter, status int, body string) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(status)
	_, _ = fmt.Fprint(resp, body)
}

func v6ValidateOptionsHandler(resp http.ResponseWriter, _ *http.Request) {
	writeJSON(resp, http.StatusOK, `{
		"vuh_usage": 0,
		"breakdown": {
			"protocol_vuh": 0,
			"browser_vuh": 0,
			"base_total_vuh": 0,
			"reduction_rate": 0,
			"reduction_rate_breakdown": null
		}
	}`)
}

func v6ProgressHandler(
	t testing.TB, testRunID int, webAppURL string, defaultProgress v6cloudapi.TestRunProgress,
	progressCallback func() v6cloudapi.TestRunProgress,
) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		tp := defaultProgress
		if progressCallback != nil {
			tp = progressCallback()
		}
		var result *string
		if tp.Result != "" {
			result = &tp.Result
		}
		writeJSON(resp, http.StatusOK,
			v6testing.TestRunJSON(t, int32(testRunID), tp.Status, result, webAppURL))
	})
}

// mockCloudOpts configures getMockCloud.
type mockCloudOpts struct {
	testRunID        int
	createHandler    http.Handler
	startHandler     http.Handler
	progressCallback func() v6cloudapi.TestRunProgress
}

func getMockCloud(t *testing.T, opts mockCloudOpts) *httptest.Server {
	if opts.testRunID == 0 {
		opts.testRunID = 123
	}
	if opts.createHandler == nil {
		opts.createHandler = cloudTestCreateSimple(t)
	}

	webAppURL := fmt.Sprintf("%s/a/k6-app/runs/%d", testStackURL, opts.testRunID)

	if opts.startHandler == nil {
		opts.startHandler = cloudTestStartSimple(t, opts.testRunID, webAppURL)
	}

	defaultProgress := v6cloudapi.TestRunProgress{
		Status:            v6cloudapi.StatusCompleted,
		EstimatedDuration: 10,
		ExecutionDuration: 10,
	}

	srv := getTestServer(t, map[string]http.Handler{
		"POST ^/cloud/v6/validate_options$":                                http.HandlerFunc(v6ValidateOptionsHandler),
		`POST ^/cloud/v6/projects/\d+/load_tests$`:                         opts.createHandler,
		`PUT ^/cloud/v6/load_tests/\d+/script$`:                            http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) { resp.WriteHeader(http.StatusNoContent) }),
		`POST ^/cloud/v6/load_tests/\d+/start$`:                            opts.startHandler,
		fmt.Sprintf("GET ^/cloud/v6/test_runs/%d$", opts.testRunID):        v6ProgressHandler(t, opts.testRunID, webAppURL, defaultProgress, opts.progressCallback),
		fmt.Sprintf("POST ^/cloud/v6/test_runs/%d/abort$", opts.testRunID): http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) { resp.WriteHeader(http.StatusNoContent) }),
		`GET ^/cloud/v6/projects/\d+/load_tests`: http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
			writeJSON(resp, http.StatusOK, fmt.Sprintf(`{"value": [%s]}`, newLoadTestJSON()))
		}),
	})

	t.Cleanup(srv.Close)
	return srv
}

func getSimpleCloudTestState(
	t *testing.T,
	script []byte,
	setupCmd setupCommandFunc,
	cliFlags []string,
	startHandler http.Handler,
	progressCallback func() v6cloudapi.TestRunProgress,
) *GlobalTestState {
	if script == nil {
		script = []byte(`export default function() {}`)
	}
	if cliFlags == nil {
		cliFlags = []string{"--verbose", "--log-output=stdout"}
	}

	srv := getMockCloud(t, mockCloudOpts{
		startHandler:     startHandler,
		progressCallback: progressCallback,
	})

	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), script, 0o644))
	ts.CmdArgs = setupCmd(cliFlags)
	ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
	ts.Env["K6_CLOUD_TOKEN"] = "foo"
	ts.Env["K6_CLOUD_STACK_ID"] = "123"
	ts.Env["K6_CLOUD_PROJECT_ID"] = "456"
	ts.Env["K6_CLOUD_STACK_URL"] = testStackURL

	return ts
}
