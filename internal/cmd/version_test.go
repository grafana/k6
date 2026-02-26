package cmd

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/cmd/tests"
)

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "--version"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotEmpty(t, stdout)

	// Check that the version/format string is correct
	assert.Contains(t, stdout, "k6 v")
	assert.Contains(t, stdout, build.Version)
	assert.Contains(t, stdout, runtime.Version())
	assert.Contains(t, stdout, runtime.GOOS)
	assert.Contains(t, stdout, runtime.GOARCH)
}

func TestVersionSubCommand(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "version"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotEmpty(t, stdout)

	// Check that the version/format string is correct
	assert.Contains(t, stdout, "k6 v")
	assert.Contains(t, stdout, build.Version)
	assert.Contains(t, stdout, runtime.Version())
	assert.Contains(t, stdout, runtime.GOOS)
	assert.Contains(t, stdout, runtime.GOARCH)
}

func TestVersionJSONSubCommand(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "version", "--json"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotEmpty(t, stdout)

	// try to unmarshal the JSON output
	var details map[string]any
	err := json.Unmarshal([]byte(stdout), &details)
	assert.NoError(t, err)

	// Check that details are correct
	assert.Contains(t, details, "version")
	assert.Contains(t, details, "go_version")
	assert.Contains(t, details, "go_os")
	assert.Contains(t, details, "go_arch")
	assert.Equal(t, "v"+build.Version, details["version"])
	assert.Equal(t, runtime.Version(), details["go_version"])
	assert.Equal(t, runtime.GOOS, details["go_os"])
	assert.Equal(t, runtime.GOARCH, details["go_arch"])
	assert.Equal(t, "devel", details[commitKey])
}

func TestVersionDetailsWithExtensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		exts        []*ext.Extension
		expected    func(t *testing.T, details map[string]any)
		expectError bool
	}{
		{
			name: "no extensions",
			exts: []*ext.Extension{},
			expected: func(t *testing.T, details map[string]any) {
				require.NotContains(t, details, "extensions")
				require.Contains(t, details, "version")
				require.Contains(t, details, "go_version")
				require.Contains(t, details, "go_os")
				require.Contains(t, details, "go_arch")
			},
		},
		{
			name: "single js extension",
			exts: []*ext.Extension{
				{
					Name:    "k6/x/test",
					Path:    "github.com/grafana/xk6-test",
					Version: "v0.1.0",
					Type:    ext.JSExtension,
				},
			},
			expected: func(t *testing.T, details map[string]any) {
				require.Contains(t, details, "extensions")
				extListRaw, ok := details["extensions"].([]any)
				require.True(t, ok, "extensions should be a slice")
				require.Len(t, extListRaw, 1)

				extMap := extListRaw[0].(map[string]any)
				require.Equal(t, "github.com/grafana/xk6-test", extMap["module"])
				require.Equal(t, "v0.1.0", extMap["version"])
				require.Equal(t, []any{"k6/x/test"}, extMap["imports"])
				require.Nil(t, extMap["outputs"])
			},
		},
		{
			name: "single output extension",
			exts: []*ext.Extension{
				{
					Name:    "prometheus",
					Path:    "github.com/grafana/xk6-output-prometheus",
					Version: "v0.2.0",
					Type:    ext.OutputExtension,
				},
			},
			expected: func(t *testing.T, details map[string]any) {
				require.Contains(t, details, "extensions")
				extListRaw, ok := details["extensions"].([]any)
				require.True(t, ok, "extensions should be a slice")
				require.Len(t, extListRaw, 1)

				extMap := extListRaw[0].(map[string]any)
				require.Equal(t, "github.com/grafana/xk6-output-prometheus", extMap["module"])
				require.Equal(t, "v0.2.0", extMap["version"])
				require.Equal(t, []any{"prometheus"}, extMap["outputs"])
				require.Nil(t, extMap["imports"])
			},
		},
		{
			name: "multiple extensions from same module",
			exts: []*ext.Extension{
				{
					Name:    "k6/x/sql",
					Path:    "github.com/grafana/xk6-sql",
					Version: "v0.3.0",
					Type:    ext.JSExtension,
				},
				{
					Name:    "k6/x/sql/driver/mysql",
					Path:    "github.com/grafana/xk6-sql",
					Version: "v0.3.0",
					Type:    ext.JSExtension,
				},
			},
			expected: func(t *testing.T, details map[string]any) {
				require.Contains(t, details, "extensions")
				extListRaw, ok := details["extensions"].([]any)
				require.True(t, ok, "extensions should be a slice")
				require.Len(t, extListRaw, 1, "should consolidate extensions from same module@version")

				extMap := extListRaw[0].(map[string]any)
				require.Equal(t, "github.com/grafana/xk6-sql", extMap["module"])
				require.Equal(t, "v0.3.0", extMap["version"])
				imports := extMap["imports"].([]any)
				require.Len(t, imports, 2)
				require.Contains(t, imports, "k6/x/sql")
				require.Contains(t, imports, "k6/x/sql/driver/mysql")
			},
		},
		{
			name: "mixed extension types from same module",
			exts: []*ext.Extension{
				{
					Name:    "k6/x/dashboard",
					Path:    "github.com/grafana/xk6-dashboard",
					Version: "v0.4.0",
					Type:    ext.JSExtension,
				},
				{
					Name:    "dashboard",
					Path:    "github.com/grafana/xk6-dashboard",
					Version: "v0.4.0",
					Type:    ext.OutputExtension,
				},
			},
			expected: func(t *testing.T, details map[string]any) {
				require.Contains(t, details, "extensions")
				extListRaw, ok := details["extensions"].([]any)
				require.True(t, ok, "extensions should be a slice")
				require.Len(t, extListRaw, 1, "should consolidate extensions from same module@version")

				extMap := extListRaw[0].(map[string]any)
				require.Equal(t, "github.com/grafana/xk6-dashboard", extMap["module"])
				require.Equal(t, "v0.4.0", extMap["version"])
				require.Equal(t, []any{"k6/x/dashboard"}, extMap["imports"])
				require.Equal(t, []any{"dashboard"}, extMap["outputs"])
			},
		},
		{
			name: "multiple different extensions",
			exts: []*ext.Extension{
				{
					Name:    "k6/x/test1",
					Path:    "github.com/example/xk6-test1",
					Version: "v1.0.0",
					Type:    ext.JSExtension,
				},
				{
					Name:    "output1",
					Path:    "github.com/example/xk6-output1",
					Version: "v2.0.0",
					Type:    ext.OutputExtension,
				},
				{
					Name:    "k6/x/test2",
					Path:    "github.com/example/xk6-test2",
					Version: "v3.0.0",
					Type:    ext.JSExtension,
				},
			},
			expected: func(t *testing.T, details map[string]any) {
				require.Contains(t, details, "extensions")
				extListRaw, ok := details["extensions"].([]any)
				require.True(t, ok, "extensions should be a slice")
				require.Len(t, extListRaw, 3)

				// Verify all extensions are present
				modules := make(map[string]bool)
				for _, extRaw := range extListRaw {
					extMap := extRaw.(map[string]any)
					modules[extMap["module"].(string)] = true
				}
				require.True(t, modules["github.com/example/xk6-test1"])
				require.True(t, modules["github.com/example/xk6-output1"])
				require.True(t, modules["github.com/example/xk6-test2"])
			},
		},
		{
			name: "same module different versions",
			exts: []*ext.Extension{
				{
					Name:    "k6/x/test",
					Path:    "github.com/example/xk6-test",
					Version: "v1.0.0",
					Type:    ext.JSExtension,
				},
				{
					Name:    "k6/x/test/v2",
					Path:    "github.com/example/xk6-test",
					Version: "v2.0.0",
					Type:    ext.JSExtension,
				},
			},
			expected: func(t *testing.T, details map[string]any) {
				require.Contains(t, details, "extensions")
				extListRaw, ok := details["extensions"].([]any)
				require.True(t, ok, "extensions should be a slice")
				require.Len(t, extListRaw, 2, "different versions should create separate entries")

				// Verify both versions are present
				versions := make(map[string]bool)
				for _, extRaw := range extListRaw {
					extMap := extRaw.(map[string]any)
					require.Equal(t, "github.com/example/xk6-test", extMap["module"])
					versions[extMap["version"].(string)] = true
				}
				require.True(t, versions["v1.0.0"])
				require.True(t, versions["v2.0.0"])
			},
		},
		{
			name: "unhandled extension type",
			exts: []*ext.Extension{
				{
					Name:    "unknown",
					Path:    "github.com/example/xk6-unknown",
					Version: "v1.0.0",
					Type:    100, // Unknown/unhandled type
				},
			},
			expected: func(_ *testing.T, _ map[string]any) {
				// Should not be called when expectError is true
			},
			expectError: true,
		},
		{
			name: "mixed handled and unhandled extensions",
			exts: []*ext.Extension{
				{
					Name:    "k6/x/test",
					Path:    "github.com/example/xk6-test",
					Version: "v1.0.0",
					Type:    ext.JSExtension,
				},
				{
					Name:    "unknown",
					Path:    "github.com/example/xk6-unknown",
					Version: "v1.0.0",
					Type:    100, // Unknown/unhandled type
				},
				{
					Name:    "output1",
					Path:    "github.com/example/xk6-output1",
					Version: "v2.0.0",
					Type:    ext.OutputExtension,
				},
			},
			expected: func(_ *testing.T, _ map[string]any) {
				// Should not be called when expectError is true
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			details, err := versionDetailsWithExtensions(tt.exts)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "unhandled extension type")
				return
			}

			require.NoError(t, err)

			// Convert to map[string]any for easier testing
			// (the actual function returns this type but nested structures need conversion)
			jsonBytes, jsonErr := json.Marshal(details)
			require.NoError(t, jsonErr)

			var result map[string]any
			jsonErr = json.Unmarshal(jsonBytes, &result)
			require.NoError(t, jsonErr)

			tt.expected(t, result)
		})
	}
}
