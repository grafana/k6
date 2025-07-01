package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/lib/fsext"
)

func TestTemplateAddCmd(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	// Set up a proper test home directory
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

	scriptPath := "test-script.js"
	require.NoError(t, fsext.WriteFile(ts.FS, scriptPath, []byte(scriptContent), 0o644))

	ts.CmdArgs = []string{"k6", "template", "add", "mytemplate", scriptPath}

	newRootCommand(ts.GlobalState).execute()

	// Check success message
	output := ts.Stdout.String()
	assert.Contains(t, output, "Template 'mytemplate' created successfully")
	assert.Contains(t, output, "k6 new --template mytemplate")

	// Verify the template was created in the correct location
	templatePath := filepath.Join(ts.UserOSConfigDir, ".k6", "templates", "mytemplate", "script.js")
	exists, err := fsext.Exists(ts.FS, templatePath)
	require.NoError(t, err)
	assert.True(t, exists, "template file should exist")

	// Verify content was copied correctly
	templateData, err := fsext.ReadFile(ts.FS, templatePath)
	require.NoError(t, err)
	assert.Equal(t, scriptContent, string(templateData))

	// Test that the template shows up in list
	ts.Stdout.Reset()
	ts.Stderr.Reset()
	ts.CmdArgs = []string{"k6", "new", "--list-templates"}
	newRootCommand(ts.GlobalState).execute()

	listOutput := ts.Stdout.String()
	assert.Contains(t, listOutput, "mytemplate")

	// Test using the newly created template
	ts.Stdout.Reset()
	ts.Stderr.Reset()
	ts.CmdArgs = []string{"k6", "new", "generated.js", "--template", "mytemplate"}
	newRootCommand(ts.GlobalState).execute()

	generatedData, err := fsext.ReadFile(ts.FS, "generated.js")
	require.NoError(t, err)
	assert.Equal(t, scriptContent, string(generatedData))
}

func TestTemplateAddCmd_NonExistentScript(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "template", "add", "mytemplate", "nonexistent.js"}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	assert.Contains(t, ts.Stderr.String(), "does not exist")
}

func TestTemplateAddCmd_InvalidTemplateName(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	// Set up a proper test home directory
	ts.UserOSConfigDir = "/test/home"
	require.NoError(t, ts.FS.MkdirAll(ts.UserOSConfigDir, 0o755))

	// Create a test script
	scriptContent := `export default function() {}`
	scriptPath := "test-script.js"
	require.NoError(t, fsext.WriteFile(ts.FS, scriptPath, []byte(scriptContent), 0o644))

	ts.CmdArgs = []string{"k6", "template", "add", "invalid/name", scriptPath}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	assert.Contains(t, ts.Stderr.String(), "cannot contain path separators")
}

func TestTemplateCmd_Help(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "template", "--help"}

	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()
	assert.Contains(t, output, "Manage k6 script templates")
	assert.Contains(t, output, "add")
}

func TestTemplateAddCmd_Help(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "template", "add", "--help"}

	newRootCommand(ts.GlobalState).execute()

	output := ts.Stdout.String()
	assert.Contains(t, output, "Promote a user script to a reusable template")
	assert.Contains(t, output, "add <name> <path>")
}
