package cmd

import (
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
	ts.ExpectedExitCode = -1

	// Create template file with invalid content in test temp directory
	templatePath := filepath.Join(t.TempDir(), "template.js")
	invalidTemplateContent := `export default function() {
  // Invalid template with {{ .InvalidField }} field
  console.log("This will cause an error");
}`
	require.NoError(t, fsext.WriteFile(ts.FS, templatePath, []byte(invalidTemplateContent), 0o600))

	ts.CmdArgs = []string{"k6", "new", "--template", templatePath, "--project-id", "9876"}
	ts.ExpectedExitCode = -1

	newRootCommand(ts.GlobalState).execute()

	assert.Contains(t, ts.Stderr.String(), "failed to execute template")

	// Verify that no script file was created
	exists, err := fsext.Exists(ts.FS, defaultNewScriptName)
	require.NoError(t, err)
	assert.False(t, exists, "script file should not exist")
}
