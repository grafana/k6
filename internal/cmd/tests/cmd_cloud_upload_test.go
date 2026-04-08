package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"go.k6.io/k6/internal/cmd"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/fsext"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestK6CloudUpload(t *testing.T) {
	t.Parallel()

	t.Run("TestCloudUploadUserNotAuthenticated", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, setupK6CloudUploadCmd, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		ts.ExpectedExitCode = -1
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `must first authenticate`)
	})

	t.Run("TestCloudUploadWithScript", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleUploadOnlyCloudTestState(t, setupK6CloudUploadCmd, nil)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://app.k6.io/a/k6-app/tests/456`)
		assert.Contains(t, stdout, `test status: Uploaded`)
	})

	// TestCloudUploadWithArchive tests that if k6 uses a static archive with the script inside that has cloud options like:
	//
	//	export let options = {
	//		cloud: {
	//			name: "my load test",
	//			projectID: 124,
	//			note: "lorem ipsum",
	//		},
	//	};
	//
	// actually sends to the cloud the archive with the correct metadata (metadata.json), like:
	//
	//	"cloud": {
	//	    "name": "my load test",
	//	    "note": "lorem ipsum",
	//	    "projectID": 124
	//	}
	t.Run("TestCloudUploadWithArchive", func(t *testing.T) {
		t.Parallel()

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
			require.Equal(t, 124, metadata.Options.Cloud.ProjectID)

			resp.Header().Set("Content-Type", "application/json")
			resp.WriteHeader(http.StatusCreated)
			_, err = fmt.Fprint(resp, newLoadTestJSON(t, "my load test"))
			assert.NoError(t, err)
		})
		srv := getTestServer(t, map[string]http.Handler{
			"POST ^/cloud/v6/validate_options$": http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
				resp.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(resp, `{}`)
				assert.NoError(t, err)
			}),
			"POST ^/cloud/v6/projects/\\d+/load_tests$": archiveUpload,
		})
		t.Cleanup(srv.Close)

		data, err := os.ReadFile(filepath.Join("testdata/archives", "archive_v1.0.0_with_cloud_option.tar")) //nolint:forbidigo // it's a test
		require.NoError(t, err)

		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), data, 0o644))

		ts.CmdArgs = []string{"k6", "cloud", "upload", "archive.tar"}
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_STACK_ID"] = "123"
		ts.Env["K6_CLOUD_STACK_URL"] = "https://app.k6.io"
		ts.Env["K6_CLOUD_PROJECT_ID"] = "124"
		ts.Env["K6_CLOUD_TOKEN"] = "foo"

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.NotContains(t, stdout, `not logged in`)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://app.k6.io/a/k6-app/tests/456`)
		assert.Contains(t, stdout, `test status: Uploaded`)
	})
}

func setupK6CloudUploadCmd(cliFlags []string) []string {
	return append([]string{"k6", "cloud", "upload"}, append(cliFlags, "test.js")...)
}
