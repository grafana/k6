/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package loader

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/testutils"
)

type errorReader string

func (e errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New((string)(e))
}

var _ io.Reader = errorReader("")

func TestReadSourceSTDINError(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	_, err := ReadSource(logger, "-", "", nil, errorReader("1234"))
	require.Error(t, err)
	require.Equal(t, "1234", err.Error())
}

func TestReadSourceSTDINCache(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	var data = []byte(`test contents`)
	var r = bytes.NewReader(data)
	var fs = afero.NewMemMapFs()
	sourceData, err := ReadSource(logger, "-", "/path/to/pwd",
		map[string]afero.Fs{"file": fsext.NewCacheOnReadFs(nil, fs, 0)}, r)
	require.NoError(t, err)
	require.Equal(t, &SourceData{
		URL:  &url.URL{Scheme: "file", Path: "/-"},
		Data: data}, sourceData)
	fileData, err := afero.ReadFile(fs, "/-")
	require.NoError(t, err)
	require.Equal(t, data, fileData)
}

func TestReadSourceRelative(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	var data = []byte(`test contents`)
	var fs = afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/path/to/somewhere/script.js", data, 0644))
	sourceData, err := ReadSource(logger, "../somewhere/script.js", "/path/to/pwd", map[string]afero.Fs{"file": fs}, nil)
	require.NoError(t, err)
	require.Equal(t, &SourceData{
		URL:  &url.URL{Scheme: "file", Path: "/path/to/somewhere/script.js"},
		Data: data}, sourceData)
}

func TestReadSourceAbsolute(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	var data = []byte(`test contents`)
	var r = bytes.NewReader(data)
	var fs = afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/a/b", data, 0644))
	require.NoError(t, afero.WriteFile(fs, "/c/a/b", []byte("wrong"), 0644))
	sourceData, err := ReadSource(logger, "/a/b", "/c", map[string]afero.Fs{"file": fs}, r)
	require.NoError(t, err)
	require.Equal(t, &SourceData{
		URL:  &url.URL{Scheme: "file", Path: "/a/b"},
		Data: data}, sourceData)
}

func TestReadSourceHttps(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	var data = []byte(`test contents`)
	var fs = afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/github.com/something", data, 0644))
	sourceData, err := ReadSource(logger, "https://github.com/something", "/c",
		map[string]afero.Fs{"file": afero.NewMemMapFs(), "https": fs}, nil)
	require.NoError(t, err)
	require.Equal(t, &SourceData{
		URL:  &url.URL{Scheme: "https", Host: "github.com", Path: "/something"},
		Data: data}, sourceData)
}

func TestReadSourceHttpError(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	var data = []byte(`test contents`)
	var fs = afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/github.com/something", data, 0644))
	_, err := ReadSource(logger, "http://github.com/something", "/c",
		map[string]afero.Fs{"file": afero.NewMemMapFs(), "https": fs}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `only supported schemes for imports are file and https`)
}

func TestReadSourceMissingFileError(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	fs := afero.NewMemMapFs()
	_, err := ReadSource(logger, "some file with spaces.js", "/c",
		map[string]afero.Fs{"file": afero.NewMemMapFs(), "https": fs}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `The moduleSpecifier "some file with spaces.js" couldn't be found on local disk.`)
}
