package cmd

import (
	"encoding/json"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/internal/lib/testutils"
)

func TestGetCmdDeps(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name              string
		files             map[string][]byte
		manifest          string
		expectedDeps      map[string]string
		expectCustomBuild bool
		expectedImports   []string
	}{
		{
			name: "single external dependency",
			files: map[string][]byte{
				"/main.js": []byte(`import http from "k6/http";
import foo from "k6/x/foo";

export default function () {
  http.get("https://example.com");
  foo();
}
`),
			},
			expectedDeps:      map[string]string{"k6": "*", "k6/x/foo": "*"},
			expectCustomBuild: true,
			expectedImports:   []string{"/main.js", "k6/http", "k6/x/foo"},
		},
		{
			name: "no external dependency",
			files: map[string][]byte{
				"/main.js": []byte(`import http from "k6/http";

export default function () {
  http.get("https://example.com");
}
`),
			},
			expectedDeps:      map[string]string{"k6": "*"},
			expectCustomBuild: false,
			expectedImports:   []string{"/main.js", "k6/http"},
		},
		{
			name: "nested local imports",
			files: map[string][]byte{
				"/main.js": []byte(`import helper from "./lib/helper.js";

export default function () {
  helper();
}
`),
				"/lib/helper.js": []byte(`import nested from "../shared/nested.js";
import ext from "k6/x/bar";

export default function () {
  nested();
  ext();
}
`),
				"/shared/nested.js": []byte(`export default function () {
  return "nested";
}
`),
			},
			expectedDeps:      map[string]string{"k6": "*", "k6/x/bar": "*"},
			expectCustomBuild: true,
			expectedImports:   []string{"/lib/helper.js", "/main.js", "/shared/nested.js", "k6/x/bar"},
		},
		{
			name: "use directive across files",
			files: map[string][]byte{
				"/main.js": []byte(`import directive from "./modules/with-directive.js";

export default function () {
  directive();
}
`),
				"/modules/with-directive.js": []byte(`"use k6 with k6/x/alpha >= 1.2.3";
import beta from "k6/x/beta";
import util from "./util.js";

export default function () {
  util();
  beta();
}
`),
				"/modules/util.js": []byte(`export default function () {
  return "util";
}
`),
			},
			expectedDeps:      map[string]string{"k6": "*", "k6/x/alpha": ">=1.2.3", "k6/x/beta": "*"},
			expectCustomBuild: true,
			expectedImports:   []string{"/main.js", "/modules/util.js", "/modules/with-directive.js", "k6/x/beta"},
		},
		{
			name: "manifest overrides default k6 constraint without use directive",
			files: map[string][]byte{
				"/main.js": []byte(`import http from "k6/http";

export default function () {
  http.get("https://example.com");
}
`),
			},
			manifest:          `{"k6": ">=1.6.0"}`,
			expectedDeps:      map[string]string{"k6": ">=1.6.0"},
			expectCustomBuild: false, // current k6 version satisfies >=1.6.0
			expectedImports:   []string{"/main.js", "k6/http"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := tests.NewGlobalTestState(t)
			ts.FS = testutils.MakeMemMapFs(t, prependCWDToFileMap(ts.Cwd, tc.files))
			ts.Flags.DependenciesManifest = tc.manifest

			cmd := getCmdDeps(ts.GlobalState)
			cmd.SetArgs([]string{"--json", "main.js"})
			require.NoError(t, cmd.Execute())

			var output struct {
				BuildDependencies   map[string]string `json:"buildDependencies"`
				Imports             []string          `json:"imports"`
				CustomBuildRequired bool              `json:"customBuildRequired"`
			}
			require.NoError(t, json.Unmarshal(ts.Stdout.Bytes(), &output))
			require.Equal(t, tc.expectedDeps, output.BuildDependencies)
			require.Equal(t, tc.expectCustomBuild, output.CustomBuildRequired)

			expectedImports := slices.Clone(tc.expectedImports)
			for i, expectedImport := range tc.expectedImports {
				if !strings.HasPrefix(expectedImport, "k6") {
					expectedImports[i] = "file://" + path.Join("/", filepath.ToSlash(ts.Cwd), expectedImport)
				}
			}

			require.EqualValues(t, expectedImports, output.Imports)
		})
	}
}

func TestGetCmdDepsHumanReadable(t *testing.T) {
	t.Parallel()
	ts := tests.NewGlobalTestState(t)
	ts.FS = testutils.MakeMemMapFs(t, prependCWDToFileMap(ts.Cwd, map[string][]byte{
		"/main.js": []byte(`import http from "k6/http";
import foo from "k6/x/foo";

export default function () {
  http.get("https://example.com");
  foo();
}
`),
	}))

	cmd := getCmdDeps(ts.GlobalState)
	cmd.SetArgs([]string{"main.js"})
	require.NoError(t, cmd.Execute())

	output := ts.Stdout.String()
	// Verify human-readable format
	require.Contains(t, output, "Build Dependencies:")
	require.Contains(t, output, "k6: *")
	require.Contains(t, output, "k6/x/foo: *")
	require.Contains(t, output, "Imports:")
	require.Contains(t, output, "Custom Build Required:")
	// Should not contain JSON structure
	require.NotContains(t, output, `"buildDependencies"`)
	require.NotContains(t, output, `"customBuildRequired"`)
}

func prependCWDToFileMap(cwd string, files map[string][]byte) map[string][]byte {
	result := make(map[string][]byte, len(files))
	for n, b := range files {
		result[cwd+n] = b
	}
	return result
}
