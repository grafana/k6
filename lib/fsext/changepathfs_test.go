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

package fsext

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestChangePathFs(t *testing.T) {
	var m = afero.NewMemMapFs()
	var prefix = "/another/"
	var c = NewChangePathFs(m, ChangePathFunc(func(name string) (string, error) {
		if !strings.HasPrefix(name, prefix) {
			return "", fmt.Errorf("path %s doesn't  start with `%s`", name, prefix)
		}
		return name[len(prefix):], nil
	}))

	var filePath = "/another/path/to/file.txt"

	require.Equal(t, c.Name(), "ChangePathFs")
	t.Run("Create", func(t *testing.T) {
		f, err := c.Create(filePath)
		require.NoError(t, err)
		require.Equal(t, filePath, f.Name())

		/** TODO figure out if this is error in MemMapFs
		_, err = c.Create(filePath)
		require.Error(t, err)
		require.True(t, os.IsExist(err))
		*/

		_, err = c.Create("/notanother/path/to/file.txt")
		checkErrorPath(t, err, "/notanother/path/to/file.txt")
	})

	t.Run("Mkdir", func(t *testing.T) {
		require.NoError(t, c.Mkdir("/another/path/too", 0644))
		checkErrorPath(t, c.Mkdir("/notanother/path/too", 0644), "/notanother/path/too")
	})

	t.Run("MkdirAll", func(t *testing.T) {
		require.NoError(t, c.MkdirAll("/another/pattth/too", 0644))
		checkErrorPath(t, c.MkdirAll("/notanother/pattth/too", 0644), "/notanother/pattth/too")
	})

	t.Run("Open", func(t *testing.T) {
		f, err := c.Open(filePath)
		require.NoError(t, err)
		require.Equal(t, filePath, f.Name())

		_, err = c.Open("/notanother/path/to/file.txt")
		checkErrorPath(t, err, "/notanother/path/to/file.txt")
	})

	t.Run("OpenFile", func(t *testing.T) {
		f, err := c.OpenFile(filePath, os.O_RDWR, 0644)
		require.NoError(t, err)
		require.Equal(t, filePath, f.Name())

		_, err = c.OpenFile("/notanother/path/to/file.txt", os.O_RDWR, 0644)
		checkErrorPath(t, err, "/notanother/path/to/file.txt")

		_, err = c.OpenFile("/another/nonexistant", os.O_RDWR, 0644)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("Stat Chmod Chtimes", func(t *testing.T) {
		info, err := c.Stat(filePath)
		require.NoError(t, err)
		require.Equal(t, "file.txt", info.Name())

		sometime := time.Unix(10000, 13)
		require.NotEqual(t, sometime, info.ModTime())
		require.NoError(t, c.Chtimes(filePath, time.Now(), sometime))
		require.Equal(t, sometime, info.ModTime())

		mode := os.FileMode(0007)
		require.NotEqual(t, mode, info.Mode())
		require.NoError(t, c.Chmod(filePath, mode))
		require.Equal(t, mode, info.Mode())

		_, err = c.Stat("/notanother/path/to/file.txt")
		checkErrorPath(t, err, "/notanother/path/to/file.txt")

		checkErrorPath(t, c.Chtimes("/notanother/path/to/file.txt", time.Now(), time.Now()), "/notanother/path/to/file.txt")

		checkErrorPath(t, c.Chmod("/notanother/path/to/file.txt", mode), "/notanother/path/to/file.txt")
	})

	t.Run("LstatIfPossible", func(t *testing.T) {
		info, ok, err := c.LstatIfPossible(filePath)
		require.NoError(t, err)
		require.False(t, ok)
		require.Equal(t, "file.txt", info.Name())

		_, _, err = c.LstatIfPossible("/notanother/path/to/file.txt")
		checkErrorPath(t, err, "/notanother/path/to/file.txt")
	})

	t.Run("Rename", func(t *testing.T) {
		info, err := c.Stat(filePath)
		require.NoError(t, err)
		require.False(t, info.IsDir())

		require.NoError(t, c.Rename(filePath, "/another/path/to/file.doc"))

		_, err = c.Stat(filePath)
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))

		info, err = c.Stat("/another/path/to/file.doc")
		require.NoError(t, err)
		require.False(t, info.IsDir())

		checkErrorPath(t,
			c.Rename("/notanother/path/to/file.txt", "/another/path/to/file.doc"),
			"/notanother/path/to/file.txt")

		checkErrorPath(t,
			c.Rename(filePath, "/notanother/path/to/file.doc"),
			"/notanother/path/to/file.doc")
	})

	t.Run("Remove", func(t *testing.T) {
		var removeFilePath = "/another/file/to/remove.txt"
		_, err := c.Create(removeFilePath)
		require.NoError(t, err)

		require.NoError(t, c.Remove(removeFilePath))

		_, err = c.Stat(removeFilePath)
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))

		_, err = c.Create(removeFilePath)
		require.NoError(t, err)

		require.NoError(t, c.RemoveAll(path.Dir(removeFilePath)))

		_, err = c.Stat(removeFilePath)
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))

		checkErrorPath(t,
			c.Remove("/notanother/path/to/file.txt"),
			"/notanother/path/to/file.txt")

		checkErrorPath(t,
			c.RemoveAll("/notanother/path/to"),
			"/notanother/path/to")
	})
}

func checkErrorPath(t *testing.T, err error, path string) {
	require.Error(t, err)
	p, ok := err.(*os.PathError)
	require.True(t, ok)
	require.Equal(t, p.Path, path)

}
