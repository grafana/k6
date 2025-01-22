package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/internal/cmd"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/fsext"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestK6CloudUpload(t *testing.T) {
	t.Parallel()

	t.Run("TestCloudUploadNotLoggedIn", func(t *testing.T) {
		t.Parallel()

		ts := getSimpleCloudTestState(t, nil, setupK6CloudUploadCmd, nil, nil, nil)
		delete(ts.Env, "K6_CLOUD_TOKEN")
		ts.ExpectedExitCode = -1
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `not logged in`)
	})

	t.Run("TestCloudUploadWithScript", func(t *testing.T) {
		t.Parallel()

		cs := func() cloudapi.TestProgressResponse {
			return cloudapi.TestProgressResponse{
				RunStatusText: "Archived",
				RunStatus:     cloudapi.RunStatusArchived,
			}
		}

		ts := getSimpleCloudTestState(t, nil, setupK6CloudUploadCmd, nil, nil, cs)
		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Archived`)
	})

	// TestCloudUploadWithArchive tests that if k6 uses a static archive with the script inside that has cloud options like:
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
	t.Run("TestCloudUploadWithArchive", func(t *testing.T) {
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

		cs := func() cloudapi.TestProgressResponse {
			return cloudapi.TestProgressResponse{
				RunStatusText: "Archived",
				RunStatus:     cloudapi.RunStatusArchived,
			}
		}

		srv := getMockCloud(t, testRunID, archiveUpload, cs)

		data, err := os.ReadFile(filepath.Join("testdata/archives", "archive_v0.46.0_with_loadimpact_option.tar")) //nolint:forbidigo // it's a test
		require.NoError(t, err)

		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), data, 0o644))

		ts.CmdArgs = []string{"k6", "cloud", "upload", "archive.tar"}
		ts.Env["K6_SHOW_CLOUD_LOGS"] = "false" // no mock for the logs yet
		ts.Env["K6_CLOUD_HOST"] = srv.URL
		ts.Env["K6_CLOUD_TOKEN"] = "foo" // doesn't matter, we mock the cloud

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		t.Log(stdout)
		assert.NotContains(t, stdout, `not logged in`)
		assert.Contains(t, stdout, `execution: cloud`)
		assert.Contains(t, stdout, `output: https://app.k6.io/runs/123`)
		assert.Contains(t, stdout, `test status: Archived`)
	})
}

func setupK6CloudUploadCmd(cliFlags []string) []string {
	return append([]string{"k6", "cloud", "upload"}, append(cliFlags, "test.js")...)
}
