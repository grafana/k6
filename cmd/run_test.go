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
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/loadimpact/k6/lib/fsext"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	walkFn := func(filePath string, info os.FileInfo, err error) error {
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

func TestAbortTest(t *testing.T) {
	// if this is not empty string this will result in spinning up REST server that
	// triggers lookup for address flag resulting in a panic.
	address = ""

	t.Run("Check status code is 107", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		logger := testutils.NewLogger(t)

		cmd := getRunCmd(ctx, logger)
		a, err := filepath.Abs("testdata/abort.js")
		require.NoError(t, err)
		cmd.SetArgs([]string{a})
		err = cmd.Execute()
		e, ok := err.(ExitCode)
		require.True(t, ok)
		require.Equal(t, abortedByScriptErrorCode, e.Code, "Status code must be 107")
		require.EqualError(t, e.error, errScriptInterrupted.Error())
	})

	t.Run("Check that teardown is called", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		msg := "Calling teardown function after abortTest()"
		var buf bytes.Buffer
		logger := logrus.New()
		logger.SetOutput(&buf)

		cmd := getRunCmd(ctx, logger)
		a, err := filepath.Abs("testdata/teardown.js")
		require.NoError(t, err)
		cmd.SetArgs([]string{a})
		err = cmd.Execute()
		e, ok := err.(ExitCode)
		require.True(t, ok)
		require.Equal(t, abortedByScriptErrorCode, e.Code, "Status code must be 107")
		require.EqualError(t, e.error, errScriptInterrupted.Error())
		require.Contains(t, buf.String(), msg)
	})

}
