package templates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/fsext"
)

func TestTemplateMetadata_Valid(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	// Create a template with valid metadata
	templateDir := "templates/myapi"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	// Create script.js
	scriptContent := `export default function() { console.log("test"); }`
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte(scriptContent), 0o644))

	// Create valid metadata
	metadata := TemplateMetadata{
		Name:            "myapi",
		Description:     "My API testing template",
		Tags:            []string{"api", "http"},
		Owner:           "test-team",
		DefaultFilename: "my-test.js",
	}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "k6.template.json"), metadataJSON, 0o644))

	// Test template manager
	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	templatesWithInfo, err := tm.ListTemplatesWithInfo()
	require.NoError(t, err)

	// Find our template
	var foundTemplate *TemplateInfo
	for _, tmpl := range templatesWithInfo {
		if tmpl.Name == "myapi" {
			foundTemplate = &tmpl
			break
		}
	}

	require.NotNil(t, foundTemplate, "Template should be found")
	require.NotNil(t, foundTemplate.Metadata, "Metadata should be parsed")
	assert.Equal(t, "My API testing template", foundTemplate.Metadata.Description)
	assert.Equal(t, []string{"api", "http"}, foundTemplate.Metadata.Tags)
	assert.Equal(t, "test-team", foundTemplate.Metadata.Owner)
	assert.Equal(t, "my-test.js", foundTemplate.Metadata.DefaultFilename)
	assert.False(t, foundTemplate.IsBuiltIn)
}

func TestTemplateMetadata_Missing(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	// Create a template without metadata
	templateDir := "templates/nometa"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	// Create script.js only
	scriptContent := `export default function() { console.log("test"); }`
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte(scriptContent), 0o644))

	// Test template manager
	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	templatesWithInfo, err := tm.ListTemplatesWithInfo()
	require.NoError(t, err)

	// Find our template
	var foundTemplate *TemplateInfo
	for _, tmpl := range templatesWithInfo {
		if tmpl.Name == "nometa" {
			foundTemplate = &tmpl
			break
		}
	}

	require.NotNil(t, foundTemplate, "Template should be found")
	assert.Nil(t, foundTemplate.Metadata, "Metadata should be nil when missing")
	assert.False(t, foundTemplate.IsBuiltIn)
}

func TestTemplateMetadata_Malformed(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	// Create a template with malformed metadata
	templateDir := "templates/badmeta"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	// Create script.js
	scriptContent := `export default function() { console.log("test"); }`
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte(scriptContent), 0o644))

	// Create malformed metadata
	malformedJSON := `{"name": "badmeta", "description": "Missing closing quote and brace`
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "k6.template.json"), []byte(malformedJSON), 0o644))

	// Test template manager
	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	templatesWithInfo, err := tm.ListTemplatesWithInfo()
	require.NoError(t, err)

	// Template should still be found but without metadata
	var foundTemplate *TemplateInfo
	for _, tmpl := range templatesWithInfo {
		if tmpl.Name == "badmeta" {
			foundTemplate = &tmpl
			break
		}
	}

	require.NotNil(t, foundTemplate, "Template should be found even with bad metadata")
	assert.Nil(t, foundTemplate.Metadata, "Metadata should be nil when malformed")
	assert.False(t, foundTemplate.IsBuiltIn)
}

func TestListTemplatesWithInfo_BuiltIns(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	templatesWithInfo, err := tm.ListTemplatesWithInfo()
	require.NoError(t, err)

	// Check that all built-in templates are included
	builtInNames := map[string]bool{
		MinimalTemplate:  false,
		ProtocolTemplate: false,
		BrowserTemplate:  false,
		RestTemplate:     false,
	}

	for _, tmpl := range templatesWithInfo {
		if _, isBuiltIn := builtInNames[tmpl.Name]; isBuiltIn {
			builtInNames[tmpl.Name] = true
			assert.True(t, tmpl.IsBuiltIn, "Built-in template should be marked as such")
			assert.Nil(t, tmpl.Metadata, "Built-in templates should not have metadata")
			assert.Empty(t, tmpl.Path, "Built-in templates should not have a path")
		}
	}

	// Verify all built-ins were found
	for name, found := range builtInNames {
		assert.True(t, found, "Built-in template %s should be found", name)
	}
}

func TestListTemplates_BackwardCompatibility(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	// Create a template with metadata
	templateDir := "templates/compat"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte("test"), 0o644))

	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	// Test backward compatibility method
	templates, err := tm.ListTemplates()
	require.NoError(t, err)

	// Should include built-ins and custom template
	expectedTemplates := []string{BrowserTemplate, "compat", MinimalTemplate, ProtocolTemplate, RestTemplate}
	assert.ElementsMatch(t, expectedTemplates, templates)
}

func TestParseTemplateMetadata_FilePermissions(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	templateDir := "templates/permtest"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	// Create metadata file
	metadata := TemplateMetadata{
		Name:        "permtest",
		Description: "Permission test template",
	}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "k6.template.json"), metadataJSON, 0o644))

	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	// Test parsing metadata directly
	parsedMetadata, err := tm.parseTemplateMetadata(templateDir)
	require.NoError(t, err)
	require.NotNil(t, parsedMetadata)
	assert.Equal(t, "Permission test template", parsedMetadata.Description)
}

func TestTemplateInfo_Sorting(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	// Create multiple templates to test sorting
	templateNames := []string{"zebra", "apple", "beta"}
	for _, name := range templateNames {
		templateDir := filepath.Join("templates", name)
		require.NoError(t, fs.MkdirAll(templateDir, 0o755))
		require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte("test"), 0o644))
	}

	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	templatesWithInfo, err := tm.ListTemplatesWithInfo()
	require.NoError(t, err)

	// Extract names and verify they're sorted
	var names []string
	for _, tmpl := range templatesWithInfo {
		names = append(names, tmpl.Name)
	}

	// Should be sorted alphabetically
	expectedOrder := []string{"apple", "beta", BrowserTemplate, MinimalTemplate, ProtocolTemplate, RestTemplate, "zebra"}
	assert.Equal(t, expectedOrder, names)
}

func TestCreateUserTemplate_AutoCreateMetadata(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	homeDir := "/test/home"
	require.NoError(t, fs.MkdirAll(homeDir, 0o755))

	// Create a script file to promote
	scriptContent := `import http from 'k6/http';
export default function() {
  http.get('https://example.com');
}`
	scriptPath := "my-script.js"
	require.NoError(t, fsext.WriteFile(fs, scriptPath, []byte(scriptContent), 0o644))

	tm, err := NewTemplateManager(fs, homeDir)
	require.NoError(t, err)

	// Create template - should auto-create metadata
	err = tm.CreateUserTemplate("my-template", scriptPath)
	require.NoError(t, err)

	// Verify template script was created
	templatePath := filepath.Join(homeDir, ".k6", "templates", "my-template", "script.js")
	exists, err := fsext.Exists(fs, templatePath)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify metadata file was auto-created
	metadataPath := filepath.Join(homeDir, ".k6", "templates", "my-template", "k6.template.json")
	exists, err = fsext.Exists(fs, metadataPath)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify metadata content
	metadataData, err := fsext.ReadFile(fs, metadataPath)
	require.NoError(t, err)

	var metadata TemplateMetadata
	require.NoError(t, json.Unmarshal(metadataData, &metadata))

	assert.Equal(t, "my-template", metadata.Name)
	assert.Equal(t, "Describe your template here.", metadata.Description)
	assert.Equal(t, []string{}, metadata.Tags)
	assert.Equal(t, "", metadata.Owner)
	assert.Equal(t, "my-script.js", metadata.DefaultFilename)
}

func TestCreateUserTemplate_DoNotOverwriteExistingMetadata(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	homeDir := "/test/home"
	require.NoError(t, fs.MkdirAll(homeDir, 0o755))

	// Create template directory with existing metadata
	templateDir := filepath.Join(homeDir, ".k6", "templates", "existing-template")
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	// Create existing metadata
	existingMetadata := TemplateMetadata{
		Name:            "existing-template",
		Description:     "Existing description",
		Tags:            []string{"existing", "tag"},
		Owner:           "existing-owner",
		DefaultFilename: "existing.js",
	}
	existingMetadataJSON, err := json.Marshal(existingMetadata)
	require.NoError(t, err)

	metadataPath := filepath.Join(templateDir, "k6.template.json")
	require.NoError(t, fsext.WriteFile(fs, metadataPath, existingMetadataJSON, 0o644))

	// Create a script file to promote
	scriptContent := `export default function() { console.log("test"); }`
	scriptPath := "new-script.js"
	require.NoError(t, fsext.WriteFile(fs, scriptPath, []byte(scriptContent), 0o644))

	tm, err := NewTemplateManager(fs, homeDir)
	require.NoError(t, err)

	// Create template - should NOT overwrite existing metadata
	err = tm.CreateUserTemplate("existing-template", scriptPath)
	require.NoError(t, err)

	// Verify metadata was not overwritten
	metadataData, err := fsext.ReadFile(fs, metadataPath)
	require.NoError(t, err)

	var metadata TemplateMetadata
	require.NoError(t, json.Unmarshal(metadataData, &metadata))

	// Should still have original values
	assert.Equal(t, "existing-template", metadata.Name)
	assert.Equal(t, "Existing description", metadata.Description)
	assert.Equal(t, []string{"existing", "tag"}, metadata.Tags)
	assert.Equal(t, "existing-owner", metadata.Owner)
	assert.Equal(t, "existing.js", metadata.DefaultFilename)
}

func TestListTemplatesWithInfo_MissingMetadataWarning(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	// Create template without metadata
	templateDir := "templates/no-metadata"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte("test"), 0o644))

	// Create template with metadata
	templateDirWithMeta := "templates/with-metadata"
	require.NoError(t, fs.MkdirAll(templateDirWithMeta, 0o755))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDirWithMeta, "script.js"), []byte("test"), 0o644))

	metadata := TemplateMetadata{
		Name:        "with-metadata",
		Description: "Has metadata",
	}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDirWithMeta, "k6.template.json"), metadataJSON, 0o644))

	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	templatesWithInfo, err := tm.ListTemplatesWithInfo()
	require.NoError(t, err)

	// Find templates
	var noMetaTemplate, withMetaTemplate, builtinTemplate *TemplateInfo
	for _, tmpl := range templatesWithInfo {
		switch tmpl.Name {
		case "no-metadata":
			noMetaTemplate = &tmpl
		case "with-metadata":
			withMetaTemplate = &tmpl
		case MinimalTemplate:
			builtinTemplate = &tmpl
		}
	}

	// Verify warnings
	require.NotNil(t, noMetaTemplate)
	assert.Equal(t, "Missing k6.template.json", noMetaTemplate.Warning)
	assert.Nil(t, noMetaTemplate.Metadata)

	require.NotNil(t, withMetaTemplate)
	assert.Empty(t, withMetaTemplate.Warning)
	assert.NotNil(t, withMetaTemplate.Metadata)

	require.NotNil(t, builtinTemplate)
	assert.Equal(t, "Missing k6.template.json", builtinTemplate.Warning)
	assert.True(t, builtinTemplate.IsBuiltIn)
}

func TestTemplateManager_IsDirectoryBasedTemplate(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	homeDir := "/test/home"
	require.NoError(t, fs.MkdirAll(homeDir, 0o755))

	// Create a directory-based template
	templateDir := "templates/multifile"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte("test"), 0o644))

	tm, err := NewTemplateManager(fs, homeDir)
	require.NoError(t, err)

	// Test built-in templates (should not be directory-based)
	assert.False(t, tm.IsDirectoryBasedTemplate("minimal"))
	assert.False(t, tm.IsDirectoryBasedTemplate("protocol"))
	assert.False(t, tm.IsDirectoryBasedTemplate("browser"))
	assert.False(t, tm.IsDirectoryBasedTemplate("rest"))

	// Test file paths (should not be directory-based)
	assert.False(t, tm.IsDirectoryBasedTemplate("./script.js"))
	assert.False(t, tm.IsDirectoryBasedTemplate("/path/to/script.js"))

	// Test directory-based template
	assert.True(t, tm.IsDirectoryBasedTemplate("multifile"))

	// Test non-existent template
	assert.False(t, tm.IsDirectoryBasedTemplate("nonexistent"))
}

func TestTemplateManager_CopyTemplateFiles(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	homeDir := "/test/home"
	require.NoError(t, fs.MkdirAll(homeDir, 0o755))

	// Create a multi-file template
	templateDir := "templates/multifile"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	// Create various files in the template
	files := map[string]string{
		"script.js":          `import http from 'k6/http';\nexport default function() { http.get('{{.ScriptName}}'); }`,
		"config.json":        `{"name": "{{.ScriptName}}", "projectId": "{{.ProjectID}}"}`,
		"README.md":          "# Test Template\n\nThis is a test template.",
		"k6.template.json":   `{"name": "multifile", "description": "Multi-file template"}`,
		".hidden":            "hidden file content",
		"subdir/helper.js":   `export function helper() { return "{{.ScriptName}}"; }`,
		"subdir/.hidden_dir": "hidden directory file",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(templateDir, filePath)
		require.NoError(t, fs.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, fsext.WriteFile(fs, fullPath, []byte(content), 0o644))
	}

	tm, err := NewTemplateManager(fs, homeDir)
	require.NoError(t, err)

	args := TemplateArgs{
		ScriptName: "my-test.js",
		ProjectID:  "12345",
	}

	var output strings.Builder
	err = tm.CopyTemplateFiles("multifile", args, false, &output)
	require.NoError(t, err)

	// Check that regular files were copied and processed
	scriptContent, err := fsext.ReadFile(fs, "script.js")
	require.NoError(t, err)
	assert.Contains(t, string(scriptContent), "my-test.js") // Template processed

	configContent, err := fsext.ReadFile(fs, "config.json")
	require.NoError(t, err)
	assert.Contains(t, string(configContent), `"name": "my-test.js"`)
	assert.Contains(t, string(configContent), `"projectId": "12345"`)

	readmeContent, err := fsext.ReadFile(fs, "README.md")
	require.NoError(t, err)
	assert.Equal(t, "# Test Template\n\nThis is a test template.", string(readmeContent))

	// Check that subdirectory was created and file copied
	helperContent, err := fsext.ReadFile(fs, "subdir/helper.js")
	require.NoError(t, err)
	assert.Contains(t, string(helperContent), "my-test.js")

	// Check that k6.template.json was NOT copied
	exists, err := fsext.Exists(fs, "k6.template.json")
	require.NoError(t, err)
	assert.False(t, exists)

	// Check that hidden files were NOT copied
	exists, err = fsext.Exists(fs, ".hidden")
	require.NoError(t, err)
	assert.False(t, exists)

	exists, err = fsext.Exists(fs, "subdir/.hidden_dir")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestTemplateManager_CopyTemplateFiles_ConflictHandling(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	homeDir := "/test/home"
	require.NoError(t, fs.MkdirAll(homeDir, 0o755))

	// Create a template
	templateDir := "templates/conflict"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte("template content"), 0o644))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "README.md"), []byte("template readme"), 0o644))

	// Create existing files
	require.NoError(t, fsext.WriteFile(fs, "script.js", []byte("existing content"), 0o644))

	tm, err := NewTemplateManager(fs, homeDir)
	require.NoError(t, err)

	args := TemplateArgs{ScriptName: "test.js", ProjectID: ""}

	// Test without overwrite - should skip existing files
	var output strings.Builder
	err = tm.CopyTemplateFiles("conflict", args, false, &output)
	require.NoError(t, err)

	// Check that warning was printed
	assert.Contains(t, output.String(), "Warning: File script.js already exists, skipping.")

	// Check that existing file was not overwritten
	content, err := fsext.ReadFile(fs, "script.js")
	require.NoError(t, err)
	assert.Equal(t, "existing content", string(content))

	// Check that non-conflicting files were still copied
	readmeContent, err := fsext.ReadFile(fs, "README.md")
	require.NoError(t, err)
	assert.Equal(t, "template readme", string(readmeContent))

	// Test with overwrite - should overwrite existing files
	output.Reset()
	err = tm.CopyTemplateFiles("conflict", args, true, &output)
	require.NoError(t, err)

	// Check that no warning was printed
	assert.NotContains(t, output.String(), "Warning:")

	// Check that existing file was overwritten
	content, err = fsext.ReadFile(fs, "script.js")
	require.NoError(t, err)
	assert.Equal(t, "template content", string(content))
}

func TestTemplateManager_CopyTemplateFiles_PreservePermissions(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	homeDir := "/test/home"
	require.NoError(t, fs.MkdirAll(homeDir, 0o755))

	// Create a template with specific permissions
	templateDir := "templates/permissions"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte("#!/usr/bin/env k6"), 0o755))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "config.json"), []byte("{}"), 0o644))

	tm, err := NewTemplateManager(fs, homeDir)
	require.NoError(t, err)

	args := TemplateArgs{ScriptName: "test.js", ProjectID: ""}

	var output strings.Builder
	err = tm.CopyTemplateFiles("permissions", args, false, &output)
	require.NoError(t, err)

	// Check that permissions were preserved
	scriptInfo, err := fs.Stat("script.js")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), scriptInfo.Mode().Perm())

	configInfo, err := fs.Stat("config.json")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), configInfo.Mode().Perm())
}

func TestTemplateManager_CopyTemplateFiles_NonTemplateFiles(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	homeDir := "/test/home"
	require.NoError(t, fs.MkdirAll(homeDir, 0o755))

	// Create a template with files that don't contain template syntax
	templateDir := "templates/notemplates"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte("plain javascript"), 0o644))
	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "data.json"), []byte(`{"key": "value"}`), 0o644))

	tm, err := NewTemplateManager(fs, homeDir)
	require.NoError(t, err)

	args := TemplateArgs{ScriptName: "test.js", ProjectID: "12345"}

	var output strings.Builder
	err = tm.CopyTemplateFiles("notemplates", args, false, &output)
	require.NoError(t, err)

	// Check that files were copied as-is without template processing
	scriptContent, err := fsext.ReadFile(fs, "script.js")
	require.NoError(t, err)
	assert.Equal(t, "plain javascript", string(scriptContent))

	dataContent, err := fsext.ReadFile(fs, "data.json")
	require.NoError(t, err)
	assert.Equal(t, `{"key": "value"}`, string(dataContent))
}

func TestTemplateManager_CopyTemplateFiles_NonDirectoryTemplate(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()
	homeDir := "/test/home"
	require.NoError(t, fs.MkdirAll(homeDir, 0o755))

	tm, err := NewTemplateManager(fs, homeDir)
	require.NoError(t, err)

	args := TemplateArgs{ScriptName: "test.js", ProjectID: ""}

	var output strings.Builder

	// Test with built-in template (should fail)
	err = tm.CopyTemplateFiles("minimal", args, false, &output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a directory-based template")

	// Test with non-existent template (should fail)
	err = tm.CopyTemplateFiles("nonexistent", args, false, &output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a directory-based template")
}

func TestTemplateArgs_TeamAndEnvVariables(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	// Create a template that uses all template variables
	templateDir := "templates/all-vars"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	templateContent := `import http from 'k6/http';

export const options = {
  vus: 1,
  duration: '10s',{{ if .Project }}
  cloud: {
    projectID: {{ .Project }},
    name: "{{ .ScriptName }}",
  },{{ end }}
  tags: { {{- if .Team }}
    team: "{{ .Team }}",{{ end }}{{ if .Env }}
    env: "{{ .Env }}",{{ end }}
  },
};

export default function() {
  // Script: {{ .ScriptName }}
  // Project: {{ .Project }} (alias: {{ .ProjectID }})
  // Team: {{ .Team }}
  // Environment: {{ .Env }}
  http.get('https://example.com');
}`

	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte(templateContent), 0o644))

	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	// Test template execution with all variables
	args := TemplateArgs{
		ScriptName: "test-script.js",
		ProjectID:  "12345",
		Project:    "12345",
		Team:       "platform-team",
		Env:        "staging",
	}

	var buf strings.Builder
	tmpl, err := tm.GetTemplate("all-vars")
	require.NoError(t, err)

	err = ExecuteTemplate(&buf, tmpl, args)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "projectID: 12345")
	assert.Contains(t, result, `team: "platform-team"`)
	assert.Contains(t, result, `env: "staging"`)
	assert.Contains(t, result, "Script: test-script.js")
	assert.Contains(t, result, "Project: 12345 (alias: 12345)")
	assert.Contains(t, result, "Team: platform-team")
	assert.Contains(t, result, "Environment: staging")
}

func TestTemplateArgs_EmptyOptionalFields(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	// Create a template with conditional rendering
	templateDir := "templates/conditional"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	templateContent := `export const options = {
  vus: 1,
  duration: '10s',{{ if .Project }}
  cloud: {
    projectID: {{ .Project }},
    name: "{{ .ScriptName }}",
  },{{ end }}
  tags: { {{- if .Team }}
    team: "{{ .Team }}",{{ end }}{{ if .Env }}
    env: "{{ .Env }}",{{ end }}
  },
};

export default function() {
  // Has team: {{ if .Team }}yes{{ else }}no{{ end }}
  // Has env: {{ if .Env }}yes{{ else }}no{{ end }}
  http.get('https://example.com');
}`

	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte(templateContent), 0o644))

	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	// Test with empty optional fields
	args := TemplateArgs{
		ScriptName: "test.js",
		ProjectID:  "",
		Project:    "",
		Team:       "",
		Env:        "",
	}

	var buf strings.Builder
	tmpl, err := tm.GetTemplate("conditional")
	require.NoError(t, err)

	err = ExecuteTemplate(&buf, tmpl, args)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Has team: no")
	assert.Contains(t, result, "Has env: no")
	assert.Contains(t, result, "tags: {")
	assert.NotContains(t, result, "cloud:")
	assert.NotContains(t, result, `team: ""`)
	assert.NotContains(t, result, `env: ""`)
}

func TestTemplateManager_CopyTemplateFiles_WithTeamAndEnv(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	// Create a multi-file template with team/env variables
	templateDir := "templates/multi-vars"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	files := map[string]string{
		"script.js": `export default function() {
  // Team: {{ .Team }}, Env: {{ .Env }}
  console.log("Running for {{ .Team }} in {{ .Env }}");
}`,
		"config.json": `{
  "name": "{{ .ScriptName }}"{{ if .Team }},
  "team": "{{ .Team }}"{{ end }}{{ if .Env }},
  "environment": "{{ .Env }}"{{ end }}
}`,
		"helpers/utils.js": `export const CONFIG = {
  team: "{{ .Team }}",
  env: "{{ .Env }}",
  script: "{{ .ScriptName }}"
};`,
	}

	for filePath, content := range files {
		fullPath := filepath.Join(templateDir, filePath)
		require.NoError(t, fs.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, fsext.WriteFile(fs, fullPath, []byte(content), 0o644))
	}

	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	args := TemplateArgs{
		ScriptName: "multi-test.js",
		ProjectID:  "",
		Project:    "",
		Team:       "qa-team",
		Env:        "production",
	}

	var output strings.Builder
	err = tm.CopyTemplateFiles("multi-vars", args, false, &output)
	require.NoError(t, err)

	// Check that all files were created and processed
	expectedFiles := map[string][]string{
		"script.js": {
			"Team: qa-team, Env: production",
			"Running for qa-team in production",
		},
		"config.json": {
			`"name": "multi-test.js"`,
			`"team": "qa-team"`,
			`"environment": "production"`,
		},
		"helpers/utils.js": {
			`team: "qa-team"`,
			`env: "production"`,
			`script: "multi-test.js"`,
		},
	}

	for file, expectedContent := range expectedFiles {
		content, err := fsext.ReadFile(fs, file)
		require.NoError(t, err)
		contentStr := string(content)

		for _, expected := range expectedContent {
			assert.Contains(t, contentStr, expected, "File %s should contain %s", file, expected)
		}
	}
}

func TestTemplateArgs_ProjectIDAndProjectAlias(t *testing.T) {
	t.Parallel()

	fs := fsext.NewMemMapFs()

	templateDir := "templates/project-alias"
	require.NoError(t, fs.MkdirAll(templateDir, 0o755))

	templateContent := `export const options = {
  cloud: {
    projectID: {{ .ProjectID }},
    project: {{ .Project }},
    name: "{{ .ScriptName }}",
  },
};

// ProjectID: {{ .ProjectID }}
// Project: {{ .Project }}`

	require.NoError(t, fsext.WriteFile(fs, filepath.Join(templateDir, "script.js"), []byte(templateContent), 0o644))

	tm, err := NewTemplateManager(fs, "")
	require.NoError(t, err)

	// Test that Project and ProjectID have the same value
	args := TemplateArgs{
		ScriptName: "alias-test.js",
		ProjectID:  "99999",
		Project:    "99999",
		Team:       "",
		Env:        "",
	}

	var buf strings.Builder
	tmpl, err := tm.GetTemplate("project-alias")
	require.NoError(t, err)

	err = ExecuteTemplate(&buf, tmpl, args)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "projectID: 99999")
	assert.Contains(t, result, "project: 99999")
	assert.Contains(t, result, "ProjectID: 99999")
	assert.Contains(t, result, "Project: 99999")
}
