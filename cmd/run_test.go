package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/cmd/tests"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/testutils"
)

type mockWriter struct {
	err      error
	errAfter int
}

func (fw mockWriter) Write(p []byte) (n int, err error) {
	if fw.err != nil {
		return fw.errAfter, fw.err
	}
	return len(p), nil
}

var _ io.Writer = mockWriter{}

func getFiles(t *testing.T, fileSystem fsext.Fs) map[string]*bytes.Buffer {
	result := map[string]*bytes.Buffer{}
	walkFn := func(filePath string, _ fs.FileInfo, err error) error {
		if filePath == "/" || filePath == "\\" {
			return nil
		}
		require.NoError(t, err)
		contents, err := fsext.ReadFile(fileSystem, filePath)
		require.NoError(t, err)
		result[filePath] = bytes.NewBuffer(contents)
		return nil
	}

	err := fsext.Walk(fileSystem, fsext.FilePathSeparator, filepath.WalkFunc(walkFn))
	require.NoError(t, err)

	return result
}

func assertEqual(t *testing.T, exp string, actual io.Reader) {
	act, err := io.ReadAll(actual)
	require.NoError(t, err)
	assert.Equal(t, []byte(exp), act)
}

func initVars() (
	content map[string]io.Reader, stdout *bytes.Buffer, stderr *bytes.Buffer, fs fsext.Fs,
) {
	return map[string]io.Reader{}, bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), fsext.NewMemMapFs()
}

func TestHandleSummaryResultSimple(t *testing.T) {
	t.Parallel()
	content, stdout, stderr, fs := initVars()

	// Test noop
	assert.NoError(t, handleSummaryResult(fs, stdout, stderr, content))
	require.Empty(t, getFiles(t, fs))
	require.Empty(t, stdout.Bytes())
	require.Empty(t, stderr.Bytes())

	// Test stdout only
	content["stdout"] = bytes.NewBufferString("some stdout summary")
	assert.NoError(t, handleSummaryResult(fs, stdout, stderr, content))
	require.Empty(t, getFiles(t, fs))
	assertEqual(t, "some stdout summary", stdout)
	require.Empty(t, stderr.Bytes())
}

func TestHandleSummaryResultError(t *testing.T) {
	t.Parallel()
	content, _, stderr, fs := initVars()

	expErr := errors.New("test error")
	stdout := mockWriter{err: expErr, errAfter: 10}

	filePath1 := "/path/file1"
	filePath2 := "/path/file2"
	if runtime.GOOS == "windows" {
		filePath1 = "\\path\\file1"
		filePath2 = "\\path\\file2"
	}

	content["stdout"] = bytes.NewBufferString("some stdout summary")
	content["stderr"] = bytes.NewBufferString("some stderr summary")
	content[filePath1] = bytes.NewBufferString("file summary 1")
	content[filePath2] = bytes.NewBufferString("file summary 2")
	err := handleSummaryResult(fs, stdout, stderr, content)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), expErr.Error())
	files := getFiles(t, fs)
	assertEqual(t, "file summary 1", files[filePath1])
	assertEqual(t, "file summary 2", files[filePath2])
}

func TestRunScriptErrorsAndAbort(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		testFilename, name   string
		expErr, expLogOutput string
		expExitCode          exitcodes.ExitCode
		extraArgs            []string
	}{
		{
			testFilename: "abort.js",
			expErr:       errext.AbortTest,
			expExitCode:  exitcodes.ScriptAborted,
		},
		{
			testFilename: "abort_initerr.js",
			expErr:       errext.AbortTest,
			expExitCode:  exitcodes.ScriptAborted,
		},
		{
			testFilename: "abort_initvu.js",
			expErr:       errext.AbortTest,
			expExitCode:  exitcodes.ScriptAborted,
		},
		{
			testFilename: "abort_teardown.js",
			expErr:       errext.AbortTest,
			expExitCode:  exitcodes.ScriptAborted,
			expLogOutput: "Calling teardown function after test.abort()",
		},
		{
			testFilename: "initerr.js",
			expErr:       "ReferenceError: someUndefinedVar is not defined",
			expExitCode:  exitcodes.ScriptException,
		},
		{
			testFilename: "thresholds/non_existing_metric.js",
			name:         "run should fail with exit status 104 on a threshold applied to a non existing metric",
			expErr:       "invalid threshold",
			expExitCode:  exitcodes.InvalidConfig,
		},
		{
			testFilename: "thresholds/non_existing_metric.js",
			name:         "run should succeed on a threshold applied to a non existing metric with the --no-thresholds flag set",
			extraArgs:    []string{"--no-thresholds"},
		},
		{
			testFilename: "thresholds/non_existing_metric.js",
			name:         "run should succeed on a threshold applied to a non existing submetric with the --no-thresholds flag set",
			extraArgs:    []string{"--no-thresholds"},
		},
		{
			testFilename: "thresholds/malformed_expression.js",
			name:         "run should fail with exit status 104 on a malformed threshold expression",
			expErr:       "malformed threshold expression",
			expExitCode:  exitcodes.InvalidConfig,
		},
		{
			testFilename: "thresholds/malformed_expression.js",
			name:         "run should on a malformed threshold expression but --no-thresholds flag set",
			extraArgs:    []string{"--no-thresholds"},
			// we don't expect an error
		},
		{
			testFilename: "thresholds/unsupported_aggregation_method.js",
			name:         "run should fail with exit status 104 on a threshold applying an unsupported aggregation method to a metric",
			expErr:       "invalid threshold",
			expExitCode:  exitcodes.InvalidConfig,
		},
		{
			testFilename: "thresholds/unsupported_aggregation_method.js",
			name:         "run should succeed on a threshold applying an unsupported aggregation method to a metric with the --no-thresholds flag set",
			extraArgs:    []string{"--no-thresholds"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		name := tc.testFilename
		if tc.name != "" {
			name = fmt.Sprintf("%s (%s)", tc.testFilename, tc.name)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testScript, err := os.ReadFile(path.Join("testdata", tc.testFilename)) //nolint:forbidigo
			require.NoError(t, err)

			ts := tests.NewGlobalTestState(t)
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, tc.testFilename), testScript, 0o644))
			ts.CmdArgs = append([]string{"k6", "run", tc.testFilename}, tc.extraArgs...)

			ts.ExpectedExitCode = int(tc.expExitCode)
			newRootCommand(ts.GlobalState).execute()

			logs := ts.LoggerHook.Drain()

			if tc.expErr != "" {
				assert.True(t, testutils.LogContains(logs, logrus.ErrorLevel, tc.expErr))
			}

			if tc.expLogOutput != "" {
				assert.True(t, testutils.LogContains(logs, logrus.InfoLevel, tc.expLogOutput))
			}
		})
	}
}

func TestInvalidOptionsThresholdErrExitCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		testFilename string
		expExitCode  exitcodes.ExitCode
		extraArgs    []string
	}{
		{
			name:         "run should fail with exit status 104 on a malformed threshold expression",
			testFilename: "thresholds/malformed_expression.js",
			expExitCode:  exitcodes.InvalidConfig,
		},
		{
			name:         "run should fail with exit status 104 on a threshold applied to a non existing metric",
			testFilename: "thresholds/non_existing_metric.js",
			expExitCode:  exitcodes.InvalidConfig,
		},
		{
			name:         "run should fail with exit status 104 on a threshold method being unsupported by the metric",
			testFilename: "thresholds/unsupported_aggregation_method.js",
			expExitCode:  exitcodes.InvalidConfig,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testScript, err := os.ReadFile(path.Join("testdata", tc.testFilename)) //nolint:forbidigo
			require.NoError(t, err)

			ts := tests.NewGlobalTestState(t)
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, tc.testFilename), testScript, 0o644))
			ts.CmdArgs = append([]string{"k6", "run", tc.testFilename}, tc.extraArgs...)

			ts.ExpectedExitCode = int(tc.expExitCode)
			newRootCommand(ts.GlobalState).execute()
		})
	}
}

func TestThresholdsRuntimeBehavior(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                 string
		testFilename         string
		expExitCode          exitcodes.ExitCode
		expStdoutContains    string
		expStdoutNotContains string
		extraArgs            []string
	}{
		{
			name:              "#2518: submetrics without values should be rendered under their parent metric #2518",
			testFilename:      "thresholds/thresholds_on_submetric_without_samples.js",
			expExitCode:       0,
			expStdoutContains: "     one..................: 0   0/s\n       { tag:xyz }........: 0   0/s\n",
		},
		{
			name:         "#2512: parsing threshold names containing parsable tokens should be valid",
			testFilename: "thresholds/name_contains_tokens.js",
			expExitCode:  0,
		},
		{
			name:                 "#2520: thresholds over metrics without values should avoid division by zero and displaying NaN values",
			testFilename:         "thresholds/empty_sink_no_nan.js",
			expExitCode:          0,
			expStdoutContains:    "rate.................: 0.00%",
			expStdoutNotContains: "NaN",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testScript, err := os.ReadFile(path.Join("testdata", tc.testFilename)) //nolint:forbidigo
			require.NoError(t, err)

			ts := tests.NewGlobalTestState(t)
			require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, tc.testFilename), testScript, 0o644))

			ts.CmdArgs = []string{"k6", "run", tc.testFilename}
			ts.ExpectedExitCode = int(tc.expExitCode)
			newRootCommand(ts.GlobalState).execute()

			if tc.expStdoutContains != "" {
				assert.Contains(t, ts.Stdout.String(), tc.expStdoutContains)
			}

			if tc.expStdoutNotContains != "" {
				t.Log(ts.Stdout.String())
				assert.NotContains(t, ts.Stdout.String(), tc.expStdoutNotContains)
			}
		})
	}
}
