package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/fsext"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cloudapi"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd"
)

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
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := getSimpleCloudTestState(t, nil, setupK6CloudRunCmd, tc.cliArgs, nil, nil)
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

		ts := makeTestState(t, script, []string{"--local-execution"}, 0)

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

		ts := makeTestState(t, script, []string{"--local-execution", "--no-archive-upload"}, 0)

		testServerHandlerFunc := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			var payload map[string]interface{}
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
}

func makeTestState(tb testing.TB, script string, cliFlags []string, expExitCode exitcodes.ExitCode) *GlobalTestState {
	if cliFlags == nil {
		cliFlags = []string{"-v", "--log-output=stdout"}
	}

	ts := NewGlobalTestState(tb)
	require.NoError(tb, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(script), 0o644))
	ts.CmdArgs = append(append([]string{"k6", "cloud", "run"}, cliFlags...), "test.js")
	ts.ExpectedExitCode = int(expExitCode)

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
