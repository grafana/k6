package cmd

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/errext/exitcodes"
)

func TestArchiveThresholds(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		noThresholds bool
		testFilename string

		wantErr bool
	}{
		{
			name:         "archive should fail with exit status 104 on a malformed threshold expression",
			noThresholds: false,
			testFilename: "testdata/thresholds/malformed_expression.js",
			wantErr:      true,
		},
		{
			name:         "archive should on a malformed threshold expression but --no-thresholds flag set",
			noThresholds: true,
			testFilename: "testdata/thresholds/malformed_expression.js",
			wantErr:      false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			testScript, err := ioutil.ReadFile(testCase.testFilename)
			require.NoError(t, err)

			testState := newGlobalTestState(t)
			require.NoError(t, afero.WriteFile(testState.fs, filepath.Join(testState.cwd, testCase.testFilename), testScript, 0o644))
			testState.args = []string{"k6", "archive", testCase.testFilename}
			if testCase.noThresholds {
				testState.args = append(testState.args, "--no-thresholds")
			}

			if testCase.wantErr {
				testState.expectedExitCode = int(exitcodes.InvalidConfig)
			}
			newRootCommand(testState.globalState).execute()
		})
	}
}
