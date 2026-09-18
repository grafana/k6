package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	provtest "go.k6.io/k6/v2/internal/cloudapi/provisioning/test"
	v6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestCloudRunLocalExecutionArchiveUsesCLIOverrides(t *testing.T) {
	t.Parallel()

	script := `
export const options = {
  cloud: { projectID: 123456 },
  vus: 10,
  duration: '30s',
};
export default function () {}
`

	ts := makeTestState(t, script, []string{"--local-execution", "--iterations", "20"})

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
		l := rc.GetLogs()
		l.SetPushUrl(srv.URL + logsPushPath)
		rc.SetLogs(l)
		resp.SetRuntimeConfig(rc)
		writeProvJSON(w, http.StatusOK, resp)
	})

	var archiveBody []byte
	var archiveUploaded atomic.Bool
	srv.HandlePresignedUpload(provtest.PresignedUploadPath, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		archiveBody = body
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

	require.True(t, archiveUploaded.Load(), "archive should have been uploaded")

	tmpPath := filepath.Join(ts.Cwd, "uploaded.tar")
	require.NoError(t, fsext.WriteFile(ts.FS, tmpPath, archiveBody, 0o644))
	require.NoError(t, testutils.Untar(t, ts.FS, tmpPath, "tmp/"))

	metadataRaw, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
	require.NoError(t, err)

	metadata := struct {
		Options struct {
			VUs        *int64          `json:"vus"`
			Iterations *int64          `json:"iterations"`
			Duration   json.RawMessage `json:"duration"`
			Scenarios  json.RawMessage `json:"scenarios"`
		} `json:"options"`
	}{}
	require.NoError(t, json.Unmarshal(metadataRaw, &metadata))

	require.NotNil(t, metadata.Options.VUs)
	assert.Equal(t, int64(10), *metadata.Options.VUs)
	require.NotNil(t, metadata.Options.Iterations)
	assert.Equal(t, int64(20), *metadata.Options.Iterations)
	assert.True(t, len(metadata.Options.Duration) == 0 || string(metadata.Options.Duration) == "null")
	assert.True(t, len(metadata.Options.Scenarios) == 0 || string(metadata.Options.Scenarios) == "null")
}
