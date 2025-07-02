package templates

import (
	"encoding/json"
	"path/filepath"
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
