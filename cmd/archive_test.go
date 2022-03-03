package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/testutils"
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

			cmd := getArchiveCmd(testutils.NewLogger(t), newCommandFlags())
			filename, err := filepath.Abs(testCase.testFilename)
			require.NoError(t, err)
			args := []string{filename}
			if testCase.noThresholds {
				args = append(args, "--no-thresholds")
			}
			cmd.SetArgs(args)
			wantExitCode := exitcodes.InvalidConfig

			var gotErrExt errext.HasExitCode
			gotErr := cmd.Execute()

			assert.Equal(t,
				testCase.wantErr,
				gotErr != nil,
				"archive command error = %v, wantErr %v", gotErr, testCase.wantErr,
			)

			if testCase.wantErr {
				require.ErrorAs(t, gotErr, &gotErrExt)
				assert.Equalf(t, wantExitCode, gotErrExt.ExitCode(),
					"status code must be %d", wantExitCode,
				)
			}
		})
	}
}
