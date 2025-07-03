package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
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

func TestNewScriptCmd_ListTemplatesWithWarnings(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template without metadata
	templateDir := "templates/no-metadata"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte("test"), 0o644))

	// Create a template with metadata
	templateDirWithMeta := "templates/with-metadata"
	require.NoError(t, ts.FS.MkdirAll(templateDirWithMeta, 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDirWithMeta, "script.js"), []byte("test"), 0o644))

	metadataContent := `{
  "name": "with-metadata",
  "description": "A template with metadata",
  "tags": ["test"],
  "owner": "test-user",
  "defaultFilename": "test.js"
}`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDirWithMeta, "k6.template.json"), []byte(metadataContent), 0o644))

	ts.CmdArgs = []string{"k6", "new", "--list-templates"}
	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()
	assert.Contains(t, output, "Available templates:")

	// Template with metadata should show description
	assert.Contains(t, output, "with-metadata - A template with metadata")

	// Template without metadata should show warning
	assert.Contains(t, output, "no-metadata (no metadata) ⚠️")

	// Built-in templates should show warning
	assert.Contains(t, output, "minimal (no metadata) ⚠️")
}

func TestNewScriptCmd_VerboseOutputWithWarnings(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template without metadata
	templateDir := "templates/warning-test"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte("test"), 0o644))

	ts.CmdArgs = []string{"k6", "new", "--list-templates", "--verbose"}
	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()

	// Should contain JSON output
	var templates []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &templates))

	// Find templates with and without warnings
	var warningTemplate, builtinTemplate *map[string]interface{}
	for _, tmpl := range templates {
		name := tmpl["Name"].(string)
		if name == "warning-test" {
			warningTemplate = &tmpl
		} else if name == "minimal" {
			builtinTemplate = &tmpl
		}
	}

	// Verify warning fields
	require.NotNil(t, warningTemplate)
	assert.Equal(t, "Missing k6.template.json", (*warningTemplate)["warning"])

	require.NotNil(t, builtinTemplate)
	assert.Equal(t, "Missing k6.template.json", (*builtinTemplate)["warning"])
	assert.Equal(t, true, (*builtinTemplate)["IsBuiltIn"])
}

func TestTemplateAddCmd_AutoCreateMetadata(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.UserOSConfigDir = "/test/home"
	require.NoError(t, ts.FS.MkdirAll(ts.UserOSConfigDir, 0o755))

	// Create a test script to promote to template
	scriptContent := `import http from 'k6/http';

export const options = {
  vus: 2,
  duration: '30s',
};

export default function() {
  http.get('https://test.example.com');
}`

	scriptPath := "auto-metadata-test.js"
	require.NoError(t, fsext.WriteFile(ts.FS, scriptPath, []byte(scriptContent), 0o644))

	ts.CmdArgs = []string{"k6", "template", "add", "auto-metadata", scriptPath}
	newRootCommand(ts.GlobalState).execute()

	// Check success message
	output := ts.Stdout.String()
	assert.Contains(t, output, "Template 'auto-metadata' created successfully")

	// Verify the template script was created
	templatePath := filepath.Join(ts.UserOSConfigDir, ".k6", "templates", "auto-metadata", "script.js")
	exists, err := fsext.Exists(ts.FS, templatePath)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify metadata file was auto-created
	metadataPath := filepath.Join(ts.UserOSConfigDir, ".k6", "templates", "auto-metadata", "k6.template.json")
	exists, err = fsext.Exists(ts.FS, metadataPath)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify metadata content
	metadataData, err := fsext.ReadFile(ts.FS, metadataPath)
	require.NoError(t, err)

	var metadata map[string]interface{}
	require.NoError(t, json.Unmarshal(metadataData, &metadata))

	assert.Equal(t, "auto-metadata", metadata["name"])
	assert.Equal(t, "Describe your template here.", metadata["description"])
	assert.Equal(t, []interface{}{}, metadata["tags"])
	assert.Equal(t, "", metadata["owner"])
	assert.Equal(t, "auto-metadata-test.js", metadata["defaultFilename"])
}

func TestTemplateAddCmd_DoNotOverwriteMetadata(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.UserOSConfigDir = "/test/home"
	require.NoError(t, ts.FS.MkdirAll(ts.UserOSConfigDir, 0o755))

	// Create existing template directory with metadata
	templateDir := filepath.Join(ts.UserOSConfigDir, ".k6", "templates", "existing")
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	// Create existing metadata
	existingMetadata := `{
  "name": "existing",
  "description": "Existing description",
  "tags": ["existing"],
  "owner": "existing-owner",
  "defaultFilename": "existing.js"
}`
	metadataPath := filepath.Join(templateDir, "k6.template.json")
	require.NoError(t, fsext.WriteFile(ts.FS, metadataPath, []byte(existingMetadata), 0o644))

	// Create a new script to add
	scriptContent := `export default function() { console.log("new script"); }`
	scriptPath := "new-script.js"
	require.NoError(t, fsext.WriteFile(ts.FS, scriptPath, []byte(scriptContent), 0o644))

	ts.CmdArgs = []string{"k6", "template", "add", "existing", scriptPath}
	newRootCommand(ts.GlobalState).execute()

	// Verify metadata was not overwritten
	metadataData, err := fsext.ReadFile(ts.FS, metadataPath)
	require.NoError(t, err)

	var metadata map[string]interface{}
	require.NoError(t, json.Unmarshal(metadataData, &metadata))

	// Should still have original values
	assert.Equal(t, "existing", metadata["name"])
	assert.Equal(t, "Existing description", metadata["description"])
	assert.Equal(t, []interface{}{"existing"}, metadata["tags"])
	assert.Equal(t, "existing-owner", metadata["owner"])
	assert.Equal(t, "existing.js", metadata["defaultFilename"])
}

func TestNewScriptCmd_DirectoryBasedTemplate_MultipleFiles(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a multi-file template directory structure
	templateDir := "templates/multifile"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	// Create multiple files in the template
	files := map[string]string{
		"script.js": `import http from 'k6/http';

export const options = {
  vus: 5,
  duration: '10s',{{ if .ProjectID }}
  cloud: {
    projectID: {{ .ProjectID }},
    name: "{{ .ScriptName }}",
  },{{ end }}
};

export default function() {
  http.get('https://example.com/api');
}`,
		"config.json": `{
  "name": "{{ .ScriptName }}",
  "description": "Multi-file template test",
  "projectId": "{{ .ProjectID }}"
}`,
		"README.md": `# {{ .ScriptName }}

This is a test script generated from a multi-file template.
`,
		"helpers/utils.js": `export function logMessage(msg) {
  console.log('{{ .ScriptName }}: ' + msg);
}`,
		"k6.template.json": `{
  "name": "multifile",
  "description": "Multi-file template for testing",
  "tags": ["test", "multifile"],
  "owner": "test-team",
  "defaultFilename": "test-script.js"
}`,
		".hidden": "This should not be copied",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(templateDir, filePath)
		require.NoError(t, ts.FS.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, fsext.WriteFile(ts.FS, fullPath, []byte(content), 0o644))
	}

	// Test using the multi-file template
	ts.CmdArgs = []string{"k6", "new", "--template", "multifile", "--project-id", "9999"}
	newRootCommand(ts.GlobalState).execute()

	// Check if there were any errors
	if ts.Stderr.Len() > 0 {
		t.Logf("Stderr: %s", ts.Stderr.String())
	}

	// Verify that all expected files were created
	expectedFiles := []string{"script.js", "config.json", "README.md", "helpers/utils.js"}
	for _, file := range expectedFiles {
		exists, err := fsext.Exists(ts.FS, file)
		require.NoError(t, err)
		assert.True(t, exists, "File %s should exist", file)

		// Check content was processed with template variables
		content, err := fsext.ReadFile(ts.FS, file)
		require.NoError(t, err)
		contentStr := string(content)

		if strings.Contains(file, ".js") || strings.Contains(file, ".json") || strings.Contains(file, ".md") {
			assert.Contains(t, contentStr, "script.js", "File %s should contain processed script name", file)
			if file == "config.json" || file == "script.js" {
				assert.Contains(t, contentStr, "9999", "File %s should contain processed project ID", file)
			}
		}
	}

	// Verify that excluded files were NOT created
	excludedFiles := []string{"k6.template.json", ".hidden"}
	for _, file := range excludedFiles {
		exists, err := fsext.Exists(ts.FS, file)
		require.NoError(t, err)
		assert.False(t, exists, "File %s should not exist", file)
	}

	// Check success message
	output := ts.Stdout.String()
	assert.Contains(t, output, "New script created from template: multifile")
}

func TestNewScriptCmd_DirectoryBasedTemplate_ConflictHandling(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template
	templateDir := "templates/conflict-test"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte("template content"), 0o644))
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "README.md"), []byte("template readme"), 0o644))

	// Create existing files
	require.NoError(t, fsext.WriteFile(ts.FS, "script.js", []byte("existing content"), 0o644))

	// Test without force flag - should show warning for conflicting files
	ts.CmdArgs = []string{"k6", "new", "--template", "conflict-test"}
	newRootCommand(ts.GlobalState).execute()

	// Check that warning was printed
	output := ts.Stdout.String()
	assert.Contains(t, output, "Warning: File script.js already exists, skipping.")

	// Check that existing file was not overwritten
	content, err := fsext.ReadFile(ts.FS, "script.js")
	require.NoError(t, err)
	assert.Equal(t, "existing content", string(content))

	// Check that non-conflicting files were still copied
	readmeContent, err := fsext.ReadFile(ts.FS, "README.md")
	require.NoError(t, err)
	assert.Equal(t, "template readme", string(readmeContent))

	// Test with force flag - should overwrite existing files
	ts.Stdout.Reset()
	ts.Stderr.Reset()
	ts.CmdArgs = []string{"k6", "new", "--template", "conflict-test", "--force"}
	newRootCommand(ts.GlobalState).execute()

	// Check that existing file was overwritten
	content, err = fsext.ReadFile(ts.FS, "script.js")
	require.NoError(t, err)
	assert.Equal(t, "template content", string(content))

	// Check that no warning was printed with force flag
	output = ts.Stdout.String()
	assert.NotContains(t, output, "Warning:")
}

func TestNewScriptCmd_DirectoryBasedTemplate_CustomScriptName(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template
	templateDir := "templates/custom-name"
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
  // Script name: {{ .ScriptName }}
  http.get('https://example.com');
}`

	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte(templateContent), 0o644))

	// Test using custom script name (should be ignored for directory-based templates)
	ts.CmdArgs = []string{"k6", "new", "custom-test.js", "--template", "custom-name", "--project-id", "1234"}
	newRootCommand(ts.GlobalState).execute()

	// For directory-based templates, the script name should still be "script.js"
	// but template variables should use the custom name
	scriptContent, err := fsext.ReadFile(ts.FS, "script.js")
	require.NoError(t, err)
	contentStr := string(scriptContent)

	assert.Contains(t, contentStr, `name: "custom-test.js"`)
	assert.Contains(t, contentStr, "projectID: 1234")
	assert.Contains(t, contentStr, "Script name: custom-test.js")

	// The custom filename should not create a separate file
	exists, err := fsext.Exists(ts.FS, "custom-test.js")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestNewScriptCmd_BuiltInTemplates_StillWork(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Test that built-in templates still work with single file creation
	builtinTemplates := []string{"minimal", "protocol", "browser", "rest"}

	for _, template := range builtinTemplates {
		t.Run(template, func(t *testing.T) {
			// Reset for each template
			ts.Stdout.Reset()
			ts.Stderr.Reset()

			// Create unique filename for each test
			filename := fmt.Sprintf("%s-test.js", template)
			ts.CmdArgs = []string{"k6", "new", filename, "--template", template}
			newRootCommand(ts.GlobalState).execute()

			// Check that single file was created with expected name
			data, err := fsext.ReadFile(ts.FS, filename)
			require.NoError(t, err)
			assert.NotEmpty(t, string(data))

			// Check success message uses old format for built-in templates
			output := ts.Stdout.String()
			assert.Contains(t, output, fmt.Sprintf("New script created: %s (%s template)", filename, template))

			// Verify it's not using directory-based logic
			assert.NotContains(t, output, "New script created from template:")
		})
	}
}

func TestNewScriptCmd_FileBasedTemplates_StillWork(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a file-based template
	templatePath := "my-template.js"
	templateContent := `export default function() {
  console.log("File-based template: {{ .ScriptName }}");
}`
	require.NoError(t, fsext.WriteFile(ts.FS, templatePath, []byte(templateContent), 0o644))

	// Test using file-based template
	ts.CmdArgs = []string{"k6", "new", "file-test.js", "--template", templatePath}
	newRootCommand(ts.GlobalState).execute()

	// Check that single file was created
	data, err := fsext.ReadFile(ts.FS, "file-test.js")
	require.NoError(t, err)
	assert.Contains(t, string(data), "File-based template: file-test.js")

	// Check success message uses old format
	output := ts.Stdout.String()
	assert.Contains(t, output, fmt.Sprintf("New script created: file-test.js (%s template)", templatePath))
	assert.NotContains(t, output, "New script created from template:")
}
