package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/cmd"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/fsext"
)

func TestK6CloudUpload(t *testing.T) {
	t.Parallel()

	t.Run("TestCloudUploadUserNotAuthenticated", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupK6CloudUploadCmd, nil, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		ts.ExpectedExitCode = -1
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `must first authenticate`)
	})

	t.Run("TestCloudUploadWithScript", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupK6CloudUploadCmd, nil, nil, nil)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, "output: "+testStackURL+"/a/k6-app/tests/456")
		assert.Contains(t, stdout, `test status: Uploaded`)
	})

	// TestCloudUploadWithArchive verifies that when uploading from a
	// pre-built archive, the K6_CLOUD_PROJECT_ID env var (456) overrides
	// the script-embedded projectID (124) in the uploaded metadata.
	t.Run("TestCloudUploadWithArchive", func(t *testing.T) {
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

		ts.CmdArgs = []string{"k6", "cloud", "upload", "archive.tar"}
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
		assert.Contains(t, stdout, "output: "+testStackURL+"/a/k6-app/tests/456")
		assert.Contains(t, stdout, `test status: Uploaded`)
	})
}

func setupK6CloudUploadCmd(cliFlags []string) []string {
	return append([]string{"k6", "cloud", "upload"}, append(cliFlags, "test.js")...)
}
