package cmd

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/lib/fsext"
)

func TestNewScriptCmd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		scriptNameArg     string
		expectedCloudName string
		expectedFilePath  string
	}{
		{
			name:              "default script name",
			expectedCloudName: "script.js",
			expectedFilePath:  defaultNewScriptName,
		},
		{
			name:              "user-specified script name",
			scriptNameArg:     "mytest.js",
			expectedCloudName: "mytest.js",
			expectedFilePath:  "mytest.js",
		},
		{
			name:              "script outside of current working directory",
			scriptNameArg:     "../mytest.js",
			expectedCloudName: "mytest.js",
			expectedFilePath:  "../mytest.js",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			ts.CmdArgs = []string{"k6", "new"}
			if testCase.scriptNameArg != "" {
				ts.CmdArgs = append(ts.CmdArgs, testCase.scriptNameArg)
			}

			newRootCommand(ts.GlobalState).execute()

			data, err := fsext.ReadFile(ts.FS, testCase.expectedFilePath)
			require.NoError(t, err)

			jsData := string(data)
			assert.Contains(t, jsData, "export const options = {")
			assert.Contains(t, jsData, "export default function() {")
		})
	}
}

func TestNewScriptCmd_FileExists_NoOverwrite(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, defaultNewScriptName, []byte("untouched"), 0o644))

	ts.CmdArgs = []string{"k6", "new"}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	data, err := fsext.ReadFile(ts.FS, defaultNewScriptName)
	require.NoError(t, err)

	assert.Contains(t, string(data), "untouched")
}

func TestNewScriptCmd_FileExists_Overwrite(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	require.NoError(t, fsext.WriteFile(ts.FS, defaultNewScriptName, []byte("untouched"), 0o644))

	ts.CmdArgs = []string{"k6", "new", "-f"}

	newRootCommand(ts.GlobalState).execute()

	data, err := fsext.ReadFile(ts.FS, defaultNewScriptName)
	require.NoError(t, err)

	assert.Contains(t, string(data), "export const options = {")
	assert.Contains(t, string(data), "export default function() {")
}

func TestNewScriptCmd_InvalidTemplateType(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "new", "--template", "invalid-template"}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()
	assert.Contains(t, ts.Stderr.String(), "invalid template type")

	// Verify that no script file was created
	exists, err := fsext.Exists(ts.FS, defaultNewScriptName)
	require.NoError(t, err)
	assert.False(t, exists, "script file should not exist")
}

func TestNewScriptCmd_ProjectID(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "new", "--project-id", "1422"}

	newRootCommand(ts.GlobalState).execute()

	data, err := fsext.ReadFile(ts.FS, defaultNewScriptName)
	require.NoError(t, err)

	assert.Contains(t, string(data), "projectID: 1422")
}

func TestNewScriptCmd_LocalTemplate(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create template file in test temp directory
	templatePath := filepath.Join(t.TempDir(), "template.js")
	templateContent := `export default function() {
  console.log("Hello, world!");
}`
	require.NoError(t, fsext.WriteFile(ts.FS, templatePath, []byte(templateContent), 0o600))

	ts.CmdArgs = []string{"k6", "new", "--template", templatePath}

	newRootCommand(ts.GlobalState).execute()

	data, err := fsext.ReadFile(ts.FS, defaultNewScriptName)
	require.NoError(t, err)

	assert.Equal(t, templateContent, string(data), "generated file should match the template content")
}

func TestNewScriptCmd_LocalTemplateWith_ProjectID(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create template file in test temp directory
	templatePath := filepath.Join(t.TempDir(), "template.js")
	templateContent := `export default function() {
  // Template with {{ .ProjectID }} project ID
  console.log("Hello from project {{ .ProjectID }}");
}`
	require.NoError(t, fsext.WriteFile(ts.FS, templatePath, []byte(templateContent), 0o600))

	ts.CmdArgs = []string{"k6", "new", "--template", templatePath, "--project-id", "9876"}

	newRootCommand(ts.GlobalState).execute()

	data, err := fsext.ReadFile(ts.FS, defaultNewScriptName)
	require.NoError(t, err)

	expectedContent := `export default function() {
  // Template with 9876 project ID
  console.log("Hello from project 9876");
}`
	assert.Equal(t, expectedContent, string(data), "generated file should have project ID interpolated")
}

func TestNewScriptCmd_LocalTemplate_NonExistentFile(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = -1

	// Use a path that we know doesn't exist in the temp directory
	nonExistentPath := filepath.Join(t.TempDir(), "nonexistent.js")

	ts.CmdArgs = []string{"k6", "new", "--template", nonExistentPath}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	assert.Contains(t, ts.Stderr.String(), "failed to read template file")

	// Verify that no script file was created
	exists, err := fsext.Exists(ts.FS, defaultNewScriptName)
	require.NoError(t, err)
	assert.False(t, exists, "script file should not exist")
}

func TestNewScriptCmd_LocalTemplate_SyntaxError(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	templatePath := filepath.Join(t.TempDir(), "template.js")
	templateContent := `export default function() {
  // Invalid template syntax 
  {{ .NotClosed 
}`
	require.NoError(t, fsext.WriteFile(ts.FS, templatePath, []byte(templateContent), 0o600))

	ts.CmdArgs = []string{"k6", "new", "--template", templatePath}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	assert.Contains(t, ts.Stderr.String(), "failed to parse template file")

	// Verify that no script file was created
	exists, err := fsext.Exists(ts.FS, defaultNewScriptName)
	require.NoError(t, err)
	assert.False(t, exists, "script file should not exist")
}

func TestNewScriptCmd_RestTemplate(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "new", "--template", "rest"}

	newRootCommand(ts.GlobalState).execute()

	data, err := fsext.ReadFile(ts.FS, defaultNewScriptName)
	require.NoError(t, err)

	jsData := string(data)
	assert.Contains(t, jsData, "REST API test")
	assert.Contains(t, jsData, "httpbin.org")
	assert.Contains(t, jsData, "export default function()")
	assert.Contains(t, jsData, "http.get")
	assert.Contains(t, jsData, "http.post")
}

func TestNewScriptCmd_ListTemplates(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "new", "--list-templates"}

	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()
	assert.Contains(t, output, "Available templates:")
	assert.Contains(t, output, "minimal")
	assert.Contains(t, output, "protocol")
	assert.Contains(t, output, "browser")
	assert.Contains(t, output, "rest")
}

func TestNewScriptCmd_LocalTemplateDirectory(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a local template directory structure
	templateDir := "templates/myapi"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	templateContent := `import http from 'k6/http';

export const options = {
  vus: 1,
  duration: '10s',
};

export default function() {
  // Local template content
  http.get('https://example.com');
}`

	templatePath := filepath.Join(templateDir, "script.js")
	require.NoError(t, fsext.WriteFile(ts.FS, templatePath, []byte(templateContent), 0o644))

	// Test listing templates includes the local one
	ts.CmdArgs = []string{"k6", "new", "--list-templates"}
	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()
	assert.Contains(t, output, "myapi")

	// Test using the local template
	ts.Stdout.Reset()
	ts.Stderr.Reset()
	ts.CmdArgs = []string{"k6", "new", "test.js", "--template", "myapi"}
	newRootCommand(ts.GlobalState).execute()

	// Check if there were any errors
	if ts.Stderr.Len() > 0 {
		t.Logf("Stderr: %s", ts.Stderr.String())
	}

	data, err := fsext.ReadFile(ts.FS, "test.js")
	require.NoError(t, err)

	jsData := string(data)
	assert.Contains(t, jsData, "Local template content")
	assert.Contains(t, jsData, "https://example.com")
}

func TestNewScriptCmd_LocalTemplateWithProjectID(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a local template directory structure
	templateDir := "templates/myapi"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	templateContent := `import http from 'k6/http';

export const options = {
  vus: 1,
  duration: '10s',{{ if .ProjectID }}
  cloud: {
    projectID: {{ .ProjectID }},
    name: "{{ .ScriptName }}",
  },{{ end }}
};

export default function() {
  // Local template with project ID
  http.get('https://example.com');
}`

	templatePath := filepath.Join(templateDir, "script.js")
	require.NoError(t, fsext.WriteFile(ts.FS, templatePath, []byte(templateContent), 0o644))

	ts.CmdArgs = []string{"k6", "new", "test.js", "--template", "myapi", "--project-id", "123"}
	newRootCommand(ts.GlobalState).execute()

	// Check if there were any errors
	if ts.Stderr.Len() > 0 {
		t.Logf("Stderr: %s", ts.Stderr.String())
	}

	data, err := fsext.ReadFile(ts.FS, "test.js")
	require.NoError(t, err)

	jsData := string(data)
	assert.Contains(t, jsData, "projectID: 123")
	assert.Contains(t, jsData, `name: "test.js"`)
}

func TestNewScriptCmd_ListTemplatesWithMetadata(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template with metadata
	templateDir := "templates/withmetadata"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	// Create script.js
	scriptContent := `export default function() { console.log("test"); }`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte(scriptContent), 0o644))

	// Create metadata file
	metadataContent := `{
  "name": "withmetadata",
  "description": "A test template with metadata",
  "tags": ["test", "metadata"],
  "owner": "test-team",
  "defaultFilename": "test-output.js"
}`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "k6.template.json"), []byte(metadataContent), 0o644))

	// Create a template without metadata for comparison
	templateDirNoMeta := "templates/nometa"
	require.NoError(t, ts.FS.MkdirAll(templateDirNoMeta, 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDirNoMeta, "script.js"), []byte(scriptContent), 0o644))

	ts.CmdArgs = []string{"k6", "new", "--list-templates"}
	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()
	assert.Contains(t, output, "Available templates:")
	assert.Contains(t, output, "withmetadata - A test template with metadata")
	assert.Contains(t, output, "nometa (no metadata)")
	assert.Contains(t, output, "minimal (no metadata)")
}

func TestNewScriptCmd_ListTemplatesVerbose(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template with metadata
	templateDir := "templates/verbose"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	// Create script.js
	scriptContent := `export default function() { console.log("verbose test"); }`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte(scriptContent), 0o644))

	// Create metadata file
	metadataContent := `{
  "name": "verbose",
  "description": "Verbose test template",
  "tags": ["verbose", "test", "json"],
  "owner": "k6-dev-team",
  "defaultFilename": "verbose-test.js"
}`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "k6.template.json"), []byte(metadataContent), 0o644))

	ts.CmdArgs = []string{"k6", "new", "--list-templates", "--verbose"}
	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()

	// Should contain JSON output (using capitalized field names)
	assert.Contains(t, output, `"Name": "verbose"`)
	assert.Contains(t, output, `"description": "Verbose test template"`)
	assert.Contains(t, output, `"tags": [`)
	assert.Contains(t, output, `"verbose"`)
	assert.Contains(t, output, `"test"`)
	assert.Contains(t, output, `"json"`)
	assert.Contains(t, output, `"owner": "k6-dev-team"`)
	assert.Contains(t, output, `"defaultFilename": "verbose-test.js"`)
	assert.Contains(t, output, `"IsBuiltIn": false`)

	// Should also contain built-in templates
	assert.Contains(t, output, `"Name": "minimal"`)
	assert.Contains(t, output, `"IsBuiltIn": true`)

	// Verify it's valid JSON
	var templates []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &templates))

	// Should have built-ins plus our custom template
	assert.GreaterOrEqual(t, len(templates), 5) // 4 built-ins + 1 custom
}

func TestNewScriptCmd_ListTemplatesVerboseEmpty(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Test with no custom templates (only built-ins)
	ts.CmdArgs = []string{"k6", "new", "--list-templates", "--verbose"}
	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()

	// Should contain JSON output with built-in templates
	var templates []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &templates))

	// Should have exactly 4 built-in templates
	assert.Len(t, templates, 4)

	// All should be built-in
	for _, tmpl := range templates {
		isBuiltIn, ok := tmpl["IsBuiltIn"]
		require.True(t, ok, "IsBuiltIn field should exist")
		if isBuiltIn != nil {
			assert.True(t, isBuiltIn.(bool), "Template should be built-in")
		}
		assert.Nil(t, tmpl["Metadata"])
		assert.Equal(t, "", tmpl["Path"])
	}
}

func TestNewScriptCmd_TemplateWithMalformedMetadata(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template with malformed metadata
	templateDir := "templates/badmeta"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	// Create script.js
	scriptContent := `export default function() { console.log("bad metadata test"); }`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte(scriptContent), 0o644))

	// Create malformed metadata file
	malformedMetadata := `{
  "name": "badmeta",
  "description": "This JSON is missing the closing brace...`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "k6.template.json"), []byte(malformedMetadata), 0o644))

	// Test that listing templates still works
	ts.CmdArgs = []string{"k6", "new", "--list-templates"}
	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()
	assert.Contains(t, output, "Available templates:")
	assert.Contains(t, output, "badmeta (no metadata)")

	// Test that the template can still be used
	ts.Stdout.Reset()
	ts.Stderr.Reset()
	ts.CmdArgs = []string{"k6", "new", "test-bad.js", "--template", "badmeta"}
	newRootCommand(ts.GlobalState).execute()

	// Should succeed despite bad metadata
	data, err := fsext.ReadFile(ts.FS, "test-bad.js")
	require.NoError(t, err)
	assert.Contains(t, string(data), "bad metadata test")
}

func TestNewScriptCmd_VerboseFlagWithoutListTemplates(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Test that --verbose flag without --list-templates works normally
	ts.CmdArgs = []string{"k6", "new", "--verbose", "--template", "minimal"}
	newRootCommand(ts.GlobalState).execute()

	// Should create a script normally
	data, err := fsext.ReadFile(ts.FS, defaultNewScriptName)
	require.NoError(t, err)
	assert.Contains(t, string(data), "export default function")

	// Output should not be JSON
	output := ts.Stdout.String()
	assert.Contains(t, output, "New script created:")
	assert.NotContains(t, output, `"name":`)
}
