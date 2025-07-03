package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/lib/fsext"
)

func TestInitProjectCmd_RequiredOrgTemplateFlag(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "init"}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	stderr := ts.Stderr.String()
	assert.Contains(t, stderr, "org-template")
}

func TestInitProjectCmd_BasicProjectScaffolding(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template for project scaffolding
	templateDir := "templates/basic-project"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	templateFiles := map[string]string{
		"script.js": `import http from 'k6/http';

export const options = {
  vus: 10,
  duration: '30s',
  tags: { {{- if .Team }}
    team: "{{ .Team }}",{{ end }}{{ if .Env }}
    env: "{{ .Env }}",{{ end }}
  },
};

export default function() {
  // Project: {{ .Name }}
  // Team: {{ .Team }}
  // Environment: {{ .Env }}
  http.get('https://api.example.com');
}`,
		"README.md": `# {{ .Name }}

This is a k6 test project for {{ .Name }}.

{{ if .Team }}Team: {{ .Team }}{{ end }}
{{ if .Env }}Environment: {{ .Env }}{{ end }}

## Usage

` + "```bash" + `
k6 run script.js
` + "```" + `
`,
		"config.json": `{
  "name": "{{ .Name }}"{{ if .Team }},
  "team": "{{ .Team }}"{{ end }}{{ if .Env }},
  "environment": "{{ .Env }}"{{ end }}
}`,
		"k6.template.json": `{
  "name": "basic-project",
  "description": "Basic project template",
  "tags": ["basic", "project"],
  "owner": "platform-team",
  "defaultFilename": "my-project",
  "type": "init"
}`,
	}

	for filePath, content := range templateFiles {
		fullPath := filepath.Join(templateDir, filePath)
		require.NoError(t, ts.FS.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, fsext.WriteFile(ts.FS, fullPath, []byte(content), 0o644))
	}

	// Test scaffolding with custom name
	ts.CmdArgs = []string{"k6", "init", "--org-template", "basic-project", "--name", "my-api-test", "--team", "backend", "--env", "staging"}
	newRootCommand(ts.GlobalState).execute()

	// Verify project directory was created
	projectExists, err := fsext.Exists(ts.FS, "my-api-test")
	require.NoError(t, err)
	assert.True(t, projectExists)

	// Verify files were copied and processed
	expectedFiles := []string{"script.js", "README.md", "config.json"}
	for _, file := range expectedFiles {
		projectFile := filepath.Join("my-api-test", file)
		content, err := fsext.ReadFile(ts.FS, projectFile)
		require.NoError(t, err)
		contentStr := string(content)

		switch file {
		case "script.js":
			assert.Contains(t, contentStr, `team: "backend"`)
			assert.Contains(t, contentStr, `env: "staging"`)
			assert.Contains(t, contentStr, "Project: my-api-test")
		case "README.md":
			assert.Contains(t, contentStr, "# my-api-test")
			assert.Contains(t, contentStr, "Team: backend")
			assert.Contains(t, contentStr, "Environment: staging")
		case "config.json":
			assert.Contains(t, contentStr, `"name": "my-api-test"`)
			assert.Contains(t, contentStr, `"team": "backend"`)
			assert.Contains(t, contentStr, `"environment": "staging"`)
		}
	}

	// Verify k6.template.json was NOT copied
	templateMetadata := filepath.Join("my-api-test", "k6.template.json")
	exists, err := fsext.Exists(ts.FS, templateMetadata)
	require.NoError(t, err)
	assert.False(t, exists)

	// Check success message
	output := ts.Stdout.String()
	assert.Contains(t, output, "✅ Project scaffolded to ./my-api-test")
	assert.Contains(t, output, "Run 'cd my-api-test && k6 run script.js' to get started")
}

func TestInitProjectCmd_DefaultProjectName(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template with defaultFilename in metadata
	templateDir := "templates/default-name"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte(`export default function() {
  console.log("Project: {{ .Name }}");
}`), 0o644))

	metadataContent := `{
  "name": "default-name",
  "description": "Template with default filename",
  "defaultFilename": "awesome-project",
  "type": "init"
}`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "k6.template.json"), []byte(metadataContent), 0o644))

	// Test without providing --name (should use defaultFilename)
	ts.CmdArgs = []string{"k6", "init", "--org-template", "default-name"}
	newRootCommand(ts.GlobalState).execute()

	// Verify project was created with default name
	projectExists, err := fsext.Exists(ts.FS, "awesome-project")
	require.NoError(t, err)
	assert.True(t, projectExists)

	// Check that template was processed
	content, err := fsext.ReadFile(ts.FS, filepath.Join("awesome-project", "script.js"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Project: awesome-project")
}

func TestInitProjectCmd_FallbackToTemplateName(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template without defaultFilename in metadata
	templateDir := "templates/fallback-name"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte(`export default function() {
  console.log("Project: {{ .Name }}");
}`), 0o644))

	// No metadata file, so should fallback to template name
	ts.CmdArgs = []string{"k6", "init", "--org-template", "fallback-name"}
	newRootCommand(ts.GlobalState).execute()

	// Verify project was created with template name
	projectExists, err := fsext.Exists(ts.FS, "fallback-name")
	require.NoError(t, err)
	assert.True(t, projectExists)

	// Check that template was processed
	content, err := fsext.ReadFile(ts.FS, filepath.Join("fallback-name", "script.js"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Project: fallback-name")
}

func TestInitProjectCmd_DirectoryAlreadyExists(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template
	templateDir := "templates/existing-test"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte("export default function() {}"), 0o644))

	// Create the target directory first
	require.NoError(t, ts.FS.MkdirAll("existing-project", 0o755))

	// Test should fail when directory exists
	ts.CmdArgs = []string{"k6", "init", "--org-template", "existing-test", "--name", "existing-project"}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	stderr := ts.Stderr.String()
	assert.Contains(t, stderr, "existing-project")
}

func TestInitProjectCmd_NonDirectoryBasedTemplate(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Test with built-in template (not directory-based)
	ts.CmdArgs = []string{"k6", "init", "--org-template", "minimal", "--name", "test-project"}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	stderr := ts.Stderr.String()
	assert.Contains(t, stderr, "directory-based template")
}

func TestInitProjectCmd_TemplateNotFound(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Test with non-existent template
	ts.CmdArgs = []string{"k6", "init", "--org-template", "non-existent", "--name", "test-project"}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	stderr := ts.Stderr.String()
	assert.Contains(t, stderr, "non-existent")
}

func TestInitProjectCmd_TemplateTypeWarnings(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a template marked as "new" type
	templateDir := "templates/new-type-template"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte("export default function() {}"), 0o644))

	metadataContent := `{
  "name": "new-type-template",
  "description": "Template for new command only",
  "type": "new"
}`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "k6.template.json"), []byte(metadataContent), 0o644))

	// Test init with "new" type template should show warning
	ts.CmdArgs = []string{"k6", "init", "--org-template", "new-type-template", "--name", "test-project"}
	newRootCommand(ts.GlobalState).execute()

	stderr := ts.Stderr.String()
	assert.Contains(t, stderr, "⚠️ This template is intended for single-script usage (k6 new). You may want to use k6 new instead.")

	// But project should still be created
	projectExists, err := fsext.Exists(ts.FS, "test-project")
	require.NoError(t, err)
	assert.True(t, projectExists)
}

func TestInitProjectCmd_AllFlags(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)

	// Create a comprehensive template
	templateDir := "templates/full-featured"
	require.NoError(t, ts.FS.MkdirAll(templateDir, 0o755))

	templateContent := `import http from 'k6/http';

export const options = {
  vus: 10,
  duration: '30s',{{ if .Project }}
  cloud: {
    projectID: {{ .Project }},
    name: "{{ .Name }}",
  },{{ end }}
  tags: { {{- if .Team }}
    team: "{{ .Team }}",{{ end }}{{ if .Env }}
    env: "{{ .Env }}",{{ end }}
  },
};

export default function() {
  // Project: {{ .Name }}
  // Team: {{ .Team }}
  // Environment: {{ .Env }}
  // ProjectID: {{ .ProjectID }}
  http.get('https://api.example.com');
}`

	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "script.js"), []byte(templateContent), 0o644))

	metadataContent := `{
  "name": "full-featured",
  "description": "Full featured template",
  "type": "both"
}`
	require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(templateDir, "k6.template.json"), []byte(metadataContent), 0o644))

	// Test with all flags
	ts.CmdArgs = []string{"k6", "init", "--org-template", "full-featured", "--name", "complete-project", "--project-id", "12345", "--team", "platform", "--env", "production"}
	newRootCommand(ts.GlobalState).execute()

	// Verify all variables were processed
	content, err := fsext.ReadFile(ts.FS, filepath.Join("complete-project", "script.js"))
	require.NoError(t, err)
	contentStr := string(content)

	assert.Contains(t, contentStr, "Project: complete-project")
	assert.Contains(t, contentStr, "Team: platform")
	assert.Contains(t, contentStr, "Environment: production")
	assert.Contains(t, contentStr, "ProjectID: 12345")
	assert.Contains(t, contentStr, "projectID: 12345")
	assert.Contains(t, contentStr, `team: "platform"`)
	assert.Contains(t, contentStr, `env: "production"`)

	// No warning for "both" type template
	stderr := ts.Stderr.String()
	assert.NotContains(t, stderr, "⚠️")
}
