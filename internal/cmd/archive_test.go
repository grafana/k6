package cmd

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib/fsext"
)

func TestArchiveThresholds(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		noThresholds bool
		testFilename string

		wantErr bool
	}{
		{
			name:         "archive should fail with exit status 104 on a malformed threshold expression",
			noThresholds: false,
			testFilename: "testdata/thresholds/malformed_expression.js",
			wantErr:      true,
		},
		{
			name:         "archive should on a malformed threshold expression but --no-thresholds flag set",
			noThresholds: true,
			testFilename: "testdata/thresholds/malformed_expression.js",
			wantErr:      false,
		},
		{
			name:         "run should fail with exit status 104 on a threshold applied to a non existing metric",
			noThresholds: false,
			testFilename: "testdata/thresholds/non_existing_metric.js",
			wantErr:      true,
		},
		{
			name:         "run should succeed on a threshold applied to a non existing metric with the --no-thresholds flag set",
			noThresholds: true,
			testFilename: "testdata/thresholds/non_existing_metric.js",
			wantErr:      false,
		},
		{
			name:         "run should succeed on a threshold applied to a non existing submetric with the --no-thresholds flag set",
			noThresholds: true,
			testFilename: "testdata/thresholds/non_existing_metric.js",
			wantErr:      false,
		},
		{
			name:         "run should fail with exit status 104 on a threshold applying an unsupported aggregation method to a metric",
			noThresholds: false,
			testFilename: "testdata/thresholds/unsupported_aggregation_method.js",
			wantErr:      true,
		},
		{
			name:         "run should succeed on a threshold applying an unsupported aggregation method to a metric with the --no-thresholds flag set",
			noThresholds: true,
			testFilename: "testdata/thresholds/unsupported_aggregation_method.js",
			wantErr:      false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testScript, err := os.ReadFile(testCase.testFilename) //nolint:forbidigo
			require.NoError(t, err)

			ts := tests.NewGlobalTestState(t)
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, testCase.testFilename), testScript, 0o644))
			ts.CmdArgs = []string{"k6", "archive", testCase.testFilename}
			if testCase.noThresholds {
				ts.CmdArgs = append(ts.CmdArgs, "--no-thresholds")
			}

			if testCase.wantErr {
				ts.ExpectedExitCode = int(exitcodes.InvalidConfig)
			}
			newRootCommand(ts.GlobalState).execute()
		})
	}
}

func TestArchiveContainsDependencies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		script       []byte
		manifest     string
		expectedDeps map[string]string
	}{
		{
			name: "no external deps, only k6",
			script: []byte(`import http from "k6/http";
export default function () { http.get("https://example.com"); }
`),
			expectedDeps: map[string]string{"k6": "*"},
		},
		{
			name: "use directive sets k6 constraint",
			script: []byte(`"use k6 >= 0.50.0";
import http from "k6/http";
export default function () {}
`),
			expectedDeps: map[string]string{"k6": ">=0.50.0"},
		},
		{
			name: "manifest overrides star constraint but pre-manifest deps are stored",
			script: []byte(`import http from "k6/http";
export default function () {}
`),
			manifest:     `{"k6": ">=1.0.0"}`,
			expectedDeps: map[string]string{"k6": "*"}, // pre-manifest: still "*"
		},
		{
			name: "manifest does not override explicit use directive constraint",
			script: []byte(`"use k6 >= 0.9.0";
import http from "k6/http";
export default function () {}
`),
			manifest:     `{"k6": ">=1.0.0"}`,
			expectedDeps: map[string]string{"k6": ">=0.9.0"}, // pre-manifest: explicit constraint preserved
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fileName := "script.js"
			ts := tests.NewGlobalTestState(t)
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, fileName), tc.script, 0o644))
			ts.Flags.DependenciesManifest = tc.manifest
			ts.CmdArgs = []string{"k6", "archive", fileName}

			newRootCommand(ts.GlobalState).execute()
			require.NoError(t, testutils.Untar(t, ts.FS, "archive.tar", "tmp/"))

			data, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
			require.NoError(t, err)

			metadata := struct {
				Dependencies map[string]string `json:"dependencies"`
			}{}
			require.NoError(t, json.Unmarshal(data, &metadata))
			require.Equal(t, tc.expectedDeps, metadata.Dependencies)
		})
	}
}

func TestArchiveContainsEnv(t *testing.T) {
	t.Parallel()

	// given some script that will be archived
	fileName := "script.js"
	testScript := []byte(`export default function () {}`)
	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, fileName), testScript, 0o644))

	// when we do archiving and passing the `--env` flags
	ts.CmdArgs = []string{"k6", "--env", "ENV1=lorem", "--env", "ENV2=ipsum", "archive", fileName}

	newRootCommand(ts.GlobalState).execute()
	require.NoError(t, testutils.Untar(t, ts.FS, "archive.tar", "tmp/"))

	data, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
	require.NoError(t, err)

	metadata := struct {
		Env map[string]string
	}{}

	// then unpacked metadata should contain the environment variables with the proper values
	require.NoError(t, json.Unmarshal(data, &metadata))
	require.Len(t, metadata.Env, 2)

	require.Contains(t, metadata.Env, "ENV1")
	require.Contains(t, metadata.Env, "ENV2")

	require.Equal(t, "lorem", metadata.Env["ENV1"])
	require.Equal(t, "ipsum", metadata.Env["ENV2"])
}

func TestArchiveContainsLegacyCloudSettings(t *testing.T) {
	t.Parallel()

	// given a script with the cloud options
	fileName := "script.js"
	testScript := []byte(`
		export let options = {
			ext: {
				loadimpact: {
					distribution: {
						one: { loadZone: 'amazon:us:ashburn', percent: 30 },
						two: { loadZone: 'amazon:ie:dublin', percent: 70 },
					},
				},
			},
		};
		export default function () {}
	`)
	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, fileName), testScript, 0o644))

	// when we do archiving
	ts.CmdArgs = []string{"k6", "archive", fileName}

	newRootCommand(ts.GlobalState).execute()
	require.NoError(t, testutils.Untar(t, ts.FS, "archive.tar", "tmp/"))

	data, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
	require.NoError(t, err)

	// we just need some basic struct
	metadata := struct {
		Options struct {
			Ext struct {
				LoadImpact struct {
					Distribution map[string]struct {
						LoadZone string  `json:"loadZone"`
						Percent  float64 `json:"percent"`
					} `json:"distribution"`
				} `json:"loadimpact"`
			} `json:"ext"`
		} `json:"options"`
	}{}

	// then unpacked metadata should contain a ext.loadimpact struct the proper values
	require.NoError(t, json.Unmarshal(data, &metadata))
	require.Len(t, metadata.Options.Ext.LoadImpact.Distribution, 2)

	require.Equal(t, "amazon:us:ashburn", metadata.Options.Ext.LoadImpact.Distribution["one"].LoadZone)
	require.Equal(t, 30., metadata.Options.Ext.LoadImpact.Distribution["one"].Percent)
	require.Equal(t, "amazon:ie:dublin", metadata.Options.Ext.LoadImpact.Distribution["two"].LoadZone)
	require.Equal(t, 70., metadata.Options.Ext.LoadImpact.Distribution["two"].Percent)
}

func TestArchiveContainsCloudSettings(t *testing.T) {
	t.Parallel()

	// given a script with the cloud options
	fileName := "script.js"
	testScript := []byte(`
		export let options = {
			cloud: {
				distribution: {
					one: { loadZone: 'amazon:us:ashburn', percent: 30 },
					two: { loadZone: 'amazon:ie:dublin', percent: 70 },
				},
			},
		};
		export default function () {}
	`)
	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, fileName), testScript, 0o644))

	// when we do archiving
	ts.CmdArgs = []string{"k6", "archive", fileName}

	newRootCommand(ts.GlobalState).execute()
	require.NoError(t, testutils.Untar(t, ts.FS, "archive.tar", "tmp/"))

	data, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
	require.NoError(t, err)

	// we just need some basic struct
	metadata := struct {
		Options struct {
			Cloud struct {
				Distribution map[string]struct {
					LoadZone string  `json:"loadZone"`
					Percent  float64 `json:"percent"`
				} `json:"distribution"`
			} `json:"cloud"`
		} `json:"options"`
	}{}

	// then unpacked metadata should contain options.cloud
	require.NoError(t, json.Unmarshal(data, &metadata))
	require.Len(t, metadata.Options.Cloud.Distribution, 2)

	require.Equal(t, "amazon:us:ashburn", metadata.Options.Cloud.Distribution["one"].LoadZone)
	require.Equal(t, 30., metadata.Options.Cloud.Distribution["one"].Percent)
	require.Equal(t, "amazon:ie:dublin", metadata.Options.Cloud.Distribution["two"].LoadZone)
	require.Equal(t, 70., metadata.Options.Cloud.Distribution["two"].Percent)
}

func TestArchiveNotContainsEnv(t *testing.T) {
	t.Parallel()

	// given some script that will be archived
	fileName := "script.js"
	testScript := []byte(`export default function () {}`)
	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, fileName), testScript, 0o644))

	// when we do archiving and passing the `--env` flags altogether with `--exclude-env-vars` flag
	ts.CmdArgs = []string{"k6", "--env", "ENV1=lorem", "--env", "ENV2=ipsum", "archive", "--exclude-env-vars", fileName}

	newRootCommand(ts.GlobalState).execute()
	require.NoError(t, testutils.Untar(t, ts.FS, "archive.tar", "tmp/"))

	data, err := fsext.ReadFile(ts.FS, "tmp/metadata.json")
	require.NoError(t, err)

	metadata := struct {
		Env map[string]string
	}{}

	// then unpacked metadata should not contain any environment variables passed at the moment of archive creation
	require.NoError(t, json.Unmarshal(data, &metadata))
	require.Len(t, metadata.Env, 0)
}

func TestArchiveStdout(t *testing.T) {
	t.Parallel()

	// given some script that will be archived
	fileName := "script.js"
	testScript := []byte(`export default function () {}`)
	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, fileName), testScript, 0o644))

	ts.CmdArgs = []string{"k6", "archive", "-O", "-", fileName}

	newRootCommand(ts.GlobalState).execute()

	for _, filename := range []string{"-", "archive.tar"} {
		_, err := ts.FS.Stat(filename)
		require.ErrorIsf(t, err, fs.ErrNotExist, "%q should not exist", filename)
	}

	require.GreaterOrEqual(t, len(ts.Stdout.Bytes()), 32)
}
