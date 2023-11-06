package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cmd/tests"
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
			assert.Contains(t, jsData, fmt.Sprintf(`name: "%s"`, testCase.expectedCloudName))
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
