/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
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

func getFiles(t *testing.T, fs afero.Fs) map[string]*bytes.Buffer {
	result := map[string]*bytes.Buffer{}
	walkFn := func(filePath string, _ os.FileInfo, err error) error {
		if filePath == "/" || filePath == "\\" {
			return nil
		}
		require.NoError(t, err)
		contents, err := afero.ReadFile(fs, filePath)
		require.NoError(t, err)
		result[filePath] = bytes.NewBuffer(contents)
		return nil
	}

	err := fsext.Walk(fs, afero.FilePathSeparator, filepath.WalkFunc(walkFn))
	require.NoError(t, err)

	return result
}

func assertEqual(t *testing.T, exp string, actual io.Reader) {
	act, err := ioutil.ReadAll(actual)
	require.NoError(t, err)
	assert.Equal(t, []byte(exp), act)
}

func initVars() (
	content map[string]io.Reader, stdout *bytes.Buffer, stderr *bytes.Buffer, fs afero.Fs,
) {
	return map[string]io.Reader{}, bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{}), afero.NewMemMapFs()
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
		expExitCode          errext.ExitCode
		extraArgs            []string
	}{
		{
			testFilename: "abort.js",
			expErr:       common.AbortTest,
			expExitCode:  exitcodes.ScriptAborted,
		},
		{
			testFilename: "abort_initerr.js",
			expErr:       common.AbortTest,
			expExitCode:  exitcodes.ScriptAborted,
		},
		{
			testFilename: "abort_initvu.js",
			expErr:       common.AbortTest,
			expExitCode:  exitcodes.ScriptAborted,
		},
		{
			testFilename: "abort_teardown.js",
			expErr:       common.AbortTest,
			expExitCode:  exitcodes.ScriptAborted,
			expLogOutput: "Calling teardown function after test.abort()",
		},
		{
			testFilename: "initerr.js",
			expErr:       "ReferenceError: someUndefinedVar is not defined",
			expExitCode:  exitcodes.ScriptException,
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
	}

	for _, tc := range testCases {
		tc := tc
		name := tc.testFilename
		if tc.name != "" {
			name = fmt.Sprintf("%s (%s)", tc.testFilename, tc.name)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testScript, err := ioutil.ReadFile(path.Join("testdata", tc.testFilename))
			require.NoError(t, err)

			testState := newGlobalTestState(t)
			require.NoError(t, afero.WriteFile(testState.fs, filepath.Join(testState.cwd, tc.testFilename), testScript, 0o644))
			testState.args = append([]string{"k6", "run", tc.testFilename}, tc.extraArgs...)

			testState.expectedExitCode = int(tc.expExitCode)
			newRootCommand(testState.globalState).execute()

			logs := testState.loggerHook.Drain()

			if tc.expErr != "" {
				assert.True(t, testutils.LogContains(logs, logrus.ErrorLevel, tc.expErr))
			}

			if tc.expLogOutput != "" {
				assert.True(t, testutils.LogContains(logs, logrus.InfoLevel, tc.expLogOutput))
			}
		})
	}
}

// alarmist is a mock module that do a panic
type alarmist struct {
	vu modules.VU
}

var _ modules.Module = &alarmist{}

func (a *alarmist) NewModuleInstance(vu modules.VU) modules.Instance {
	return &alarmist{
		vu: vu,
	}
}

func (a *alarmist) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"panic": a.panic,
		},
	}
}

func (a *alarmist) panic(s string) {
	panic(s)
}

func TestRunScriptPanicsErrorsAndAbort(t *testing.T) {
	t.Parallel()

	modules.Register("k6/x/alarmist", new(alarmist))

	testCases := []struct {
		caseName, testScript, expectedLogMessage string
	}{
		{
			caseName: "panic in the VU context",
			testScript: `
			import { panic } from 'k6/x/alarmist';

			export default function() {
				panic('hey')
			}
			`,
			expectedLogMessage: "a panic occurred in VU code: hey",
		},
		{
			caseName: "panic in the init context",
			testScript: `
			import { panic } from 'k6/x/alarmist';

			panic('hey')
			export default function() {
				console.log('lorem ipsum');
			}
			`,
			expectedLogMessage: "a panic occurred in the init context code: hey",
		},
	}

	for _, tc := range testCases {
		tc := tc
		name := tc.caseName

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testFilename := "script.js"
			testState := newGlobalTestState(t)
			require.NoError(t, afero.WriteFile(testState.fs, filepath.Join(testState.cwd, testFilename), []byte(tc.testScript), 0o644))
			testState.args = []string{"k6", "run", testFilename}

			testState.expectedExitCode = int(exitcodes.ScriptAborted)
			newRootCommand(testState.globalState).execute()

			logs := testState.loggerHook.Drain()

			assert.True(t, testutils.LogContains(logs, logrus.ErrorLevel, tc.expectedLogMessage))
		})
	}
}
