package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/errext/exitcodes"
	provtest "go.k6.io/k6/v2/internal/cloudapi/provisioning/test"
	v6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/cloudapi/v6/v6test"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestK6CloudRun(t *testing.T) {
	t.Parallel()
	runCloudTests(t, setupK6CloudRunCmd)
}

func setupK6CloudRunCmd(cliFlags []string) []string {
	return append([]string{"k6", "cloud", "run"}, append(cliFlags, "test.js")...)
}

// TestCloudRunWithArchive tests that if k6 uses a static archive with the script inside that has cloud options like:
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
//	"cloud": {
//	    "name": "my load test",
//	    "note": "lorem ipsum",
//	    "projectID": 124
//	}
func TestCloudRunWithArchive(t *testing.T) {
	t.Parallel()

	ts := NewGlobalTestState(t)

	inspectArchive := func(req *http.Request) {
		// v6 API uses "script" as the multipart field name (v1 used "file").
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
		require.Equal(t, 124, metadata.Options.Cloud.ProjectID)
	}

	srv := v6test.NewServer(t, v6test.Config{
		InspectArchive: inspectArchive,
	})

	data, err := os.ReadFile(filepath.Join("testdata/archives", "archive_v1.0.0_with_cloud_option.tar")) //nolint:forbidigo // it's a test
	require.NoError(t, err)

	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), data, 0o644))

	ts.CmdArgs = []string{"k6", "cloud", "run", "--verbose", "--log-output=stdout", "archive.tar"}
	ts.Env["K6_SHOW_CLOUD_LOGS"] = "false" // no mock for the logs yet
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
	ts.Env["K6_CLOUD_TOKEN"] = "foo" // doesn't matter, we mock the cloud
	ts.Env["K6_CLOUD_STACK_ID"] = "1"

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotContains(t, stdout, `not logged in`)
	assert.Contains(t, stdout, `execution: cloud`)
	assert.Contains(t, stdout, `hello world from archive`)
	assert.Contains(t, stdout, `output: https://stack.grafana.com/a/k6-app/runs/123`)
	assert.Contains(t, stdout, `test status: Finished`)
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
			name:               "--secret-source=cloud is not a valid value",
			cliArgs:            []string{"--secret-source=cloud"},
			wantStderrContains: "'cloud' is not a valid value for --secret-source",
		},
		{
			name:               "--secret-source=cloud is not a valid value even with --local-execution",
			cliArgs:            []string{"--local-execution", "--secret-source=cloud"},
			wantStderrContains: "'cloud' is not a valid value for --secret-source",
		},
		{
			name:               "using --no-cloud-secrets without --local-execution should fail",
			cliArgs:            []string{"--no-cloud-secrets"},
			wantStderrContains: "the --no-cloud-secrets flag can only be used in conjunction with the --local-execution flag",
		},
		{
			name:               "using --no-cloud-logs without --local-execution should fail",
			cliArgs:            []string{"--no-cloud-logs"},
			wantStderrContains: "the --no-cloud-logs flag can only be used in conjunction with the --local-execution flag",
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

	t.Run("should upload the test archive via presigned S3 URL as a default", func(t *testing.T) {
		t.Parallel()

		script := `
export const options = {
  cloud: {
      name: 'Hello k6 Cloud!',
      projectID: 123456,
  },
};

export default function() {};`

		ts := makeTestState(t, script, []string{"--local-execution"})

		srv := provtest.NewServer(t)

		// v6 CreateOrFindLoadTest: POST /cloud/v6/projects/{projectID}/load_tests
		srv.HandleCreateLoadTest(123456, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			res := k6cloud.NewLoadTestApiModelWithDefaults()
			res.SetId(provtest.DefaultLoadTestID)
			writeProvJSON(w, http.StatusCreated, res)
		})

		// start_local_execution: check request body and return response
		var startCalled atomic.Bool
		srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, r *http.Request) {
			startCalled.Store(true)

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))

			// Verify options field is present and contains resolved lib.Options
			assert.Contains(t, payload, "options")

			// Verify archive_size > 0
			assert.Contains(t, payload, "archive_size")
			archiveSize, ok := payload["archive_size"].(float64)
			assert.True(t, ok, "archive_size should be a number")
			assert.Greater(t, archiveSize, float64(0), "archive_size should be > 0")

			resp := provtest.DefaultStartLocalExecutionResponse()
			// Override URLs to point to the test server
			uploadURL := srv.URL + provtest.PresignedUploadPath
			resp.SetArchiveUploadUrl(uploadURL)
			resp.SetTestRunDetailsPageUrl(fmt.Sprintf("%s/runs/%d", srv.URL, provtest.DefaultTestRunID))
			// Override metrics push URL to point to the test server
			rc := resp.GetRuntimeConfig()
			m := rc.GetMetrics()
			m.SetPushUrl(srv.URL + "/v1/metrics")
			rc.SetMetrics(m)
			resp.SetRuntimeConfig(rc)
			writeProvJSON(w, http.StatusOK, resp)
		})

		// presigned archive upload
		var archiveUploaded atomic.Bool
		srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, r *http.Request) {
			archiveUploaded.Store(true)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Greater(t, len(body), 0, "archive upload body should not be empty")
			assert.Equal(t, "application/x-tar", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
		})

		// v6 FetchTestRun: return "initializing" immediately
		srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
			{Status: v6.StatusInitializing},
		})

		// notify: verify it's called
		var notifyCalled atomic.Bool
		srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, _ *http.Request) {
			notifyCalled.Store(true)
			w.WriteHeader(http.StatusOK)
		})

		// catch-all for metrics pushes and other calls
		srv.Mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, fmt.Sprintf("output: cloud (%s/runs/%d)", srv.URL, provtest.DefaultTestRunID))
		assert.True(t, startCalled.Load(), "start_local_execution should have been called")
		assert.True(t, archiveUploaded.Load(), "archive should have been uploaded via presigned URL")
		assert.True(t, notifyCalled.Load(), "notify should have been called at test end")
	})

	t.Run("does not upload the archive when --no-archive-upload is provided", func(t *testing.T) {
		t.Parallel()

		script := `
export const options = {
  cloud: {
      name: 'Hello k6 Cloud!',
      projectID: 123456,
  },
};

export default function() {};`

		ts := makeTestState(t, script, []string{"--local-execution", "--no-archive-upload"})

		srv := provtest.NewServer(t)

		srv.HandleCreateLoadTest(123456, func(w http.ResponseWriter, _ *http.Request) {
			res := k6cloud.NewLoadTestApiModelWithDefaults()
			res.SetId(provtest.DefaultLoadTestID)
			writeProvJSON(w, http.StatusCreated, res)
		})

		// start_local_execution: verify archive_size is null
		srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))

			// archive_size should be explicitly null (Go JSON unmarshals null as nil)
			assert.Contains(t, payload, "archive_size")
			assert.Nil(t, payload["archive_size"], "archive_size should be null when --no-archive-upload is provided")

			resp := provtest.DefaultStartLocalExecutionResponse()
			// No upload URL when --no-archive-upload is set
			resp.ArchiveUploadUrl.Unset()
			resp.SetTestRunDetailsPageUrl(fmt.Sprintf("%s/runs/%d", srv.URL, provtest.DefaultTestRunID))
			rc := resp.GetRuntimeConfig()
			m := rc.GetMetrics()
			m.SetPushUrl(srv.URL + "/v1/metrics")
			rc.SetMetrics(m)
			resp.SetRuntimeConfig(rc)
			writeProvJSON(w, http.StatusOK, resp)
		})

		// Archive upload should NOT be called
		var archiveUploaded atomic.Bool
		srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
			archiveUploaded.Store(true)
			w.WriteHeader(http.StatusOK)
		})

		srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
			{Status: v6.StatusInitializing},
		})

		srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		srv.Mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, fmt.Sprintf("output: cloud (%s/runs/%d)", srv.URL, provtest.DefaultTestRunID))
		assert.False(t, archiveUploaded.Load(), "archive should NOT have been uploaded when --no-archive-upload is set")
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

		srv := provtest.NewServer(t)

		srv.HandleCreateLoadTest(123456, func(w http.ResponseWriter, _ *http.Request) {
			res := k6cloud.NewLoadTestApiModelWithDefaults()
			res.SetId(provtest.DefaultLoadTestID)
			writeProvJSON(w, http.StatusCreated, res)
		})

		srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
			resp := provtest.DefaultStartLocalExecutionResponse()
			uploadURL := srv.URL + provtest.PresignedUploadPath
			resp.SetArchiveUploadUrl(uploadURL)
			resp.SetTestRunDetailsPageUrl(fmt.Sprintf("%s/runs/%d", srv.URL, provtest.DefaultTestRunID))
			rc := resp.GetRuntimeConfig()
			m := rc.GetMetrics()
			m.SetPushUrl(srv.URL + "/v1/metrics")
			rc.SetMetrics(m)
			resp.SetRuntimeConfig(rc)
			writeProvJSON(w, http.StatusOK, resp)
		})

		srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
			{Status: v6.StatusInitializing},
		})

		srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		srv.Mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, fmt.Sprintf("output: cloud (%s/runs/%d)", srv.URL, provtest.DefaultTestRunID))
		assert.Contains(t, stdout, "The test run id is "+strconv.Itoa(int(provtest.DefaultTestRunID)))
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
			"POST ^/provisioning/v1/": http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				require.Fail(t, "provisioning API must not be called when K6_CLOUD_PUSH_REF_ID is set")
			}),
			"POST ^/cloud/v6/projects/": http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				require.Fail(t, "v6 load_tests API must not be called when K6_CLOUD_PUSH_REF_ID is set")
			}),
		})
		t.Cleanup(srv.Close)
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)

		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, "output: cloud (https://app.k6.io/runs/"+pushRefID+")")
		assert.Contains(t, stdout, "The test run id is "+pushRefID)
	})
}

func TestCloudRunLocalExecutionNoCloudSecrets(t *testing.T) {
	t.Parallel()

	script := `
export const options = {
  cloud: {
      name: 'Test no-cloud-secrets',
      projectID: 123456,
  },
};
export default function() {};`

	ts := makeTestState(t, script, []string{"--local-execution", "--no-cloud-secrets"})

	srv := provtest.NewServer(t)

	srv.HandleCreateLoadTest(123456, func(w http.ResponseWriter, _ *http.Request) {
		res := k6cloud.NewLoadTestApiModelWithDefaults()
		res.SetId(provtest.DefaultLoadTestID)
		writeProvJSON(w, http.StatusCreated, res)
	})

	srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		resp := provtest.DefaultStartLocalExecutionResponse()
		uploadURL := srv.URL + provtest.PresignedUploadPath
		resp.SetArchiveUploadUrl(uploadURL)
		resp.SetTestRunDetailsPageUrl(fmt.Sprintf("%s/runs/%d", srv.URL, provtest.DefaultTestRunID))
		rc := resp.GetRuntimeConfig()
		m := rc.GetMetrics()
		m.SetPushUrl(srv.URL + "/v1/metrics")
		rc.SetMetrics(m)
		resp.SetRuntimeConfig(rc)
		writeProvJSON(w, http.StatusOK, resp)
	})

	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv.Mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	// --no-cloud-secrets must prevent the cloud source from being registered.
	assert.Nil(t, ts.CloudSecretSource, "cloud secret source should not be registered when --no-cloud-secrets is set")
}

func TestCloudRunLocalExecutionCloudLogPusher(t *testing.T) {
	t.Parallel()

	script := `
export const options = {
  cloud: {
      name: 'Test cloud logs',
      projectID: 123456,
  },
};
export default function() {};`

	t.Run("registers the pusher for --local-execution", func(t *testing.T) {
		t.Parallel()

		ts := makeTestState(t, script, []string{"--local-execution"})
		setupLocalExecutionProvMock(t, ts)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		assert.NotNil(t, ts.CloudLogPusher,
			"cloud log pusher should be registered for k6 cloud run --local-execution")
	})

	t.Run("does not register the pusher with --no-cloud-logs", func(t *testing.T) {
		t.Parallel()

		ts := makeTestState(t, script, []string{"--local-execution", "--no-cloud-logs"})
		setupLocalExecutionProvMock(t, ts)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		assert.Nil(t, ts.CloudLogPusher,
			"cloud log pusher should not be registered when --no-cloud-logs is set")
	})

	t.Run("does not register the pusher for non-local-execution", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(script), 0o644))
		ts.CmdArgs = []string{"k6", "run", "test.js"}

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		assert.Nil(t, ts.CloudLogPusher,
			"cloud log pusher should not be registered for a non-local-execution run")
	})
}

// setupLocalExecutionProvMock wires a provisioning mock server for a
// k6 cloud run --local-execution flow and points ts at it. It mirrors the
// handlers used by TestCloudRunLocalExecutionNoCloudSecrets.
func setupLocalExecutionProvMock(t *testing.T, ts *GlobalTestState) {
	t.Helper()

	srv := provtest.NewServer(t)

	srv.HandleCreateLoadTest(123456, func(w http.ResponseWriter, _ *http.Request) {
		res := k6cloud.NewLoadTestApiModelWithDefaults()
		res.SetId(provtest.DefaultLoadTestID)
		writeProvJSON(w, http.StatusCreated, res)
	})

	srv.HandleStartLocalExecution(provtest.DefaultLoadTestID, func(w http.ResponseWriter, _ *http.Request) {
		resp := provtest.DefaultStartLocalExecutionResponse()
		uploadURL := srv.URL + provtest.PresignedUploadPath
		resp.SetArchiveUploadUrl(uploadURL)
		resp.SetTestRunDetailsPageUrl(fmt.Sprintf("%s/runs/%d", srv.URL, provtest.DefaultTestRunID))
		rc := resp.GetRuntimeConfig()
		m := rc.GetMetrics()
		m.SetPushUrl(srv.URL + "/v1/metrics")
		rc.SetMetrics(m)
		resp.SetRuntimeConfig(rc)
		writeProvJSON(w, http.StatusOK, resp)
	})

	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv.HandleFetchTestRun(provtest.DefaultTestRunID, []v6.TestProgress{
		{Status: v6.StatusInitializing},
	})

	srv.HandleNotify(provtest.DefaultTestRunID, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv.Mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts.Env["K6_CLOUD_HOST"] = srv.URL
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
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

func writeProvJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(fmt.Errorf("writeProvJSON: encoding JSON: %w", err))
	}
}
