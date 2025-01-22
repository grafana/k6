package cmd

import (
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
		testCase := testCase
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
