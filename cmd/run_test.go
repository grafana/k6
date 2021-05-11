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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/fsext"
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
