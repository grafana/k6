package tests

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cloudapi/v6/v6test"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib/fsext"
)

const cloudFeaturesDefaultScript = `
export const options = {
  cloud: { name: 'feature flags', projectID: 124 },
};
export default function () {};`

func uploadAndCaptureArchive(
	t *testing.T, cmdArgs []string, env map[string]string, script string,
) (map[string]string, []byte) {
	t.Helper()

	ts := NewGlobalTestState(t)

	var (
		mu          sync.Mutex
		archiveEnv  map[string]string
		archiveRaw  []byte
		wasUploaded bool
	)
	inspectArchive := func(req *http.Request) {
		file, _, err := req.FormFile("script")
		require.NoError(t, err)
		data, err := io.ReadAll(file)
		require.NoError(t, err)

		tmpPath := filepath.Join(ts.Cwd, "archive_to_cloud.tar")
		require.NoError(t, fsext.WriteFile(ts.FS, tmpPath, data, 0o644))
		require.NoError(t, testutils.Untar(t, ts.FS, tmpPath, "tmp/"))

		metadataRaw, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
		require.NoError(t, err)

		metadata := struct {
			Env map[string]string `json:"env"`
		}{}
		require.NoError(t, json.Unmarshal(metadataRaw, &metadata))

		mu.Lock()
		archiveEnv = metadata.Env
		archiveRaw = data
		wasUploaded = true
		mu.Unlock()
	}

	srv := v6test.NewServer(t, v6test.Config{InspectArchive: inspectArchive})

	if script == "" {
		script = cloudFeaturesDefaultScript
	}
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "test.js"), []byte(script), 0o644))

	ts.CmdArgs = cmdArgs
	ts.Env["K6_SHOW_CLOUD_LOGS"] = "false"
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL
	ts.Env["K6_CLOUD_TOKEN"] = "foo"
	ts.Env["K6_CLOUD_STACK_ID"] = "1"
	maps.Copy(ts.Env, env)

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	mu.Lock()
	defer mu.Unlock()
	require.True(t, wasUploaded, "cloud upload must have happened")
	return archiveEnv, archiveRaw
}

func TestCloudRunPropagatesFeatureFlagsToWorker(t *testing.T) {
	t.Parallel()

	archiveEnv, archive := uploadAndCaptureArchive(t,
		[]string{"k6", "cloud", "run", "--features", "native-histograms", "test.js"}, nil, "")

	assert.Equal(t, "native-histograms", archiveEnv["K6_FEATURES"])

	// replay the baked archive on a worker: it must activate the feature offline
	ts := NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), archive, 0o644))
	ts.CmdArgs = []string{
		"k6", "run", "--include-system-env-vars=false", "--no-usage-report",
		"--out", "json=results.json", "archive.tar",
	}

	tagged := runAndNativeHistTagged(t, ts)
	assert.NotEmpty(t, tagged, "worker running the uploaded archive must activate the baked feature")
}

func TestCloudRunClearsInheritedFeatureFlags(t *testing.T) {
	t.Parallel()

	archiveEnv, _ := uploadAndCaptureArchive(t,
		[]string{"k6", "cloud", "run", "-e", "K6_FEATURES=native-histograms", "--features", "", "test.js"}, nil, "")

	_, ok := archiveEnv["K6_FEATURES"]
	assert.False(t, ok, "explicit clear must drop the inherited K6_FEATURES")
}

func TestCloudRunPropagatesFeatureFlagFromEnv(t *testing.T) {
	t.Parallel()

	// K6_FEATURES set in the process environment (not -e, not --features) must be
	// honored by the cloud run path, even though it defaults --include-system-env-vars
	// to false. The env surface is the user's environment, not the filtered runtime env.
	archiveEnv, _ := uploadAndCaptureArchive(t,
		[]string{"k6", "cloud", "run", "test.js"},
		map[string]string{"K6_FEATURES": "native-histograms"}, "")

	assert.Equal(t, "native-histograms", archiveEnv["K6_FEATURES"])
}

func TestCloudRunPropagatesFeatureFlagFromLegacyAliasEnv(t *testing.T) {
	t.Parallel()

	// A legacy alias env var set in the process environment must resolve to its
	// canonical name and propagate as K6_FEATURES on the cloud run path.
	archiveEnv, _ := uploadAndCaptureArchive(t,
		[]string{"k6", "cloud", "run", "test.js"},
		map[string]string{"K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM": "true"}, "")

	assert.Equal(t, "native-histograms", archiveEnv["K6_FEATURES"])
}

func TestCloudRunIgnoresFeatureFlagFromScriptOptions(t *testing.T) {
	t.Parallel()

	// Features in the script's exported options are unsupported, so they must
	// not convey to the worker as K6_FEATURES.
	script := `
export const options = {
  cloud: { name: 'feature flags', projectID: 124 },
  features: ['native-histograms'],
};
export default function () {};`
	archiveEnv, _ := uploadAndCaptureArchive(t,
		[]string{"k6", "cloud", "run", "test.js"}, nil, script)

	_, ok := archiveEnv["K6_FEATURES"]
	assert.False(t, ok, "options.features must not convey to the worker")
}
