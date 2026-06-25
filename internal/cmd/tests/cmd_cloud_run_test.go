package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/errext/exitcodes"
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

	t.Run("should upload the test archive with a multipart request as a default", func(t *testing.T) {
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

		testServerHandlerFunc := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// When using the local execution mode, the test archive should be uploaded to the cloud
			// as a multipart request.
			formData, err := parseMultipartRequest(req)
			require.NoError(t, err, "expected a correctly formed multipart request")
			assert.Contains(t, formData, "name")
			assert.Equal(t, "Hello k6 Cloud!", formData["name"])
			assert.Contains(t, formData, "project_id")
			assert.Equal(t, "123456", formData["project_id"])
			assert.Contains(t, formData, "file")
			assert.NotEmpty(t, formData["file"])

			resp.WriteHeader(http.StatusOK)
			_, err = fmt.Fprint(resp, `{
			"reference_id": "1337",
			"test_run_token": "mock-test-run-token",
			"secrets_config": {
				"endpoint": "https://mock-secrets.example.com/{key}",
				"response_path": "plaintext"
			},
			"config": {
				"testRunDetails": "https://some.other.url/foo/tests/org/1337?bar=baz"
			}
		}`)
			assert.NoError(t, err)
		})

		srv := getCloudTestEndChecker(t, 1337, testServerHandlerFunc, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed)
		ts.Env["K6_CLOUD_HOST"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, "output: cloud (https://some.other.url/foo/tests/org/1337?bar=baz)")
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

		testServerHandlerFunc := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			var payload map[string]any
			err = json.Unmarshal(body, &payload)
			require.NoError(t, err)

			assert.Contains(t, payload, "name")
			assert.Equal(t, "Hello k6 Cloud!", payload["name"])
			assert.Contains(t, payload, "project_id")
			assert.Equal(t, float64(123456), payload["project_id"])
			assert.NotContains(t, payload, "file")

			resp.WriteHeader(http.StatusOK)
			_, err = fmt.Fprint(resp, `{
			"reference_id": "1337",
			"test_run_token": "mock-test-run-token",
			"secrets_config": {
				"endpoint": "https://mock-secrets.example.com/{key}",
				"response_path": "plaintext"
			},
			"config": {
				"testRunDetails": "https://some.other.url/foo/tests/org/1337?bar=baz"
			}
		}`)
			assert.NoError(t, err)
		})

		srv := getCloudTestEndChecker(t, 1337, testServerHandlerFunc, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed)
		ts.Env["K6_CLOUD_HOST"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, "output: cloud (https://some.other.url/foo/tests/org/1337?bar=baz)")
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

		const testRunID = 1337
		srv := getCloudTestEndChecker(t, testRunID, nil, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed)
		ts.Env["K6_CLOUD_HOST"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, "execution: local")
		assert.Contains(t, stdout, "output: cloud (https://app.k6.io/runs/1337)")
		assert.Contains(t, stdout, "The test run id is "+strconv.Itoa(testRunID))
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

	testServerHandlerFunc := http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		resp.WriteHeader(http.StatusOK)
		_, err := fmt.Fprint(resp, `{
			"reference_id": "1337",
			"test_run_token": "mock-test-run-token",
			"secrets_config": {
				"endpoint": "https://mock-secrets.example.com/{key}",
				"response_path": "plaintext"
			},
			"config": {
				"testRunDetails": "https://some.other.url/foo/tests/org/1337?bar=baz"
			}
		}`)
		assert.NoError(t, err)
	})

	srv := getCloudTestEndChecker(t, 1337, testServerHandlerFunc, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed)
	ts.Env["K6_CLOUD_HOST"] = srv.URL

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	// --no-cloud-secrets must prevent the cloud source from being registered.
	assert.Nil(t, ts.CloudSecretSource, "cloud secret source should not be registered when --no-cloud-secrets is set")
}

func TestCloudRunLocalExecutionPreservesSingleExplicitSecretSourceDefault(t *testing.T) {
	t.Parallel()

	script := `
import secrets from "k6/secrets";

export const options = {
  cloud: {
      name: 'Test local secret source default',
      projectID: 123456,
  },
};

export default async function() {
	const secret = await secrets.get("file-secret");
	console.log(secret);
};`

	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(script), 0o644))
	secretsFile := filepath.Join(ts.Cwd, "secrets.env")
	require.NoError(t, fsext.WriteFile(ts.FS, secretsFile, []byte("file-secret=file-secret-value\n"), 0o644))

	testServerHandlerFunc := http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		resp.WriteHeader(http.StatusOK)
		_, err := fmt.Fprint(resp, `{
			"reference_id": "1337",
			"test_run_token": "mock-test-run-token",
			"secrets_config": {
				"endpoint": "https://mock-secrets.example.com/{key}",
				"response_path": "plaintext"
			},
			"config": {
				"testRunDetails": "https://some.other.url/foo/tests/org/1337?bar=baz"
			}
		}`)
		assert.NoError(t, err)
	})

	srv := getCloudTestEndChecker(t, 1337, testServerHandlerFunc, cloudapi.RunStatusFinished, cloudapi.ResultStatusPassed)
	ts.CmdArgs = []string{"k6", "cloud", "run", "--local-execution", "--log-output=stdout", "--secret-source=file=" + secretsFile, "test.js"}
	ts.Env["K6_CLOUD_TOKEN"] = "foo"
	ts.Env["K6_CLOUD_STACK_ID"] = "1234"
	ts.Env["K6_CLOUD_HOST"] = srv.URL

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotContains(t, stdout, "level=error")
	assert.Contains(t, stdout, `level=info msg="***SECRET_REDACTED***" source=console`)
	assert.NotContains(t, stdout, "file-secret-value")
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

func parseMultipartRequest(r *http.Request) (map[string]string, error) {
	// Parse the multipart form data
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}

	// Initialize a map to store the parsed form data
	formData := make(map[string]string)

	// Iterate through the parts
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, err
		}

		// Read the part content
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, part)
		if err != nil {
			return nil, err
		}

		// Store the part content in the map
		formData[part.FormName()] = buf.String()
	}

	return formData, nil
}
