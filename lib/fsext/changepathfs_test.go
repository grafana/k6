package fsext

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestChangePathFs(t *testing.T) {
	t.Parallel()
	filePath := "/another/path/to/file.txt"
	newTestFS := func() *ChangePathFs {
		m := afero.NewMemMapFs()
		prefix := "/another/"
		c := NewChangePathFs(m, ChangePathFunc(func(name string) (string, error) {
			if !strings.HasPrefix(name, prefix) {
				return "", fmt.Errorf("path %s doesn't  start with `%s`", name, prefix)
			}
			return name[len(prefix):], nil
		}))

		require.Equal(t, c.Name(), "ChangePathFs")
		f, err := c.Create(filePath)
		require.NoError(t, err)
		require.Equal(t, filePath, f.Name())
		return c
	}
	t.Run("Create", func(t *testing.T) {
		t.Parallel()
		c := newTestFS()

		/** TODO figure out if this is error in MemMapFs
		_, err = c.Create(filePath)
		require.Error(t, err)
		require.True(t, os.IsExist(err))
		*/

		_, err := c.Create("/notanother/path/to/file.txt")
		checkErrorPath(t, err, "/notanother/path/to/file.txt")
	})

	t.Run("Mkdir", func(t *testing.T) {
		t.Parallel()
		c := newTestFS()
		require.NoError(t, c.Mkdir("/another/path/too", 0o644))
		checkErrorPath(t, c.Mkdir("/notanother/path/too", 0o644), "/notanother/path/too")
	})

	t.Run("MkdirAll", func(t *testing.T) {
		t.Parallel()
		c := newTestFS()
		require.NoError(t, c.MkdirAll("/another/pattth/too", 0o644))
		checkErrorPath(t, c.MkdirAll("/notanother/pattth/too", 0o644), "/notanother/pattth/too")
	})

	t.Run("Open", func(t *testing.T) {
		t.Parallel()
		c := newTestFS()
		f, err := c.Open(filePath)
		require.NoError(t, err)
		require.Equal(t, filePath, f.Name())

		_, err = c.Open("/notanother/path/to/file.txt")
		checkErrorPath(t, err, "/notanother/path/to/file.txt")
	})

	t.Run("OpenFile", func(t *testing.T) {
		t.Parallel()
		c := newTestFS()
		f, err := c.OpenFile(filePath, syscall.O_RDWR, 0o644)
		require.NoError(t, err)
		require.Equal(t, filePath, f.Name())

		_, err = c.OpenFile("/notanother/path/to/file.txt", syscall.O_RDWR, 0o644)
		checkErrorPath(t, err, "/notanother/path/to/file.txt")

		_, err = c.OpenFile("/another/nonexistant", syscall.O_RDWR, 0o644)
		require.True(t, errors.Is(err, fs.ErrNotExist))
	})

	t.Run("Stat Chmod Chtimes", func(t *testing.T) {
		t.Parallel()
		c := newTestFS()
		info, err := c.Stat(filePath)
		require.NoError(t, err)
		require.Equal(t, "file.txt", info.Name())

		sometime := time.Unix(10000, 13)
		require.NotEqual(t, sometime, info.ModTime())
		require.NoError(t, c.Chtimes(filePath, time.Now(), sometime))
		require.Equal(t, sometime, info.ModTime())

		mode := fs.FileMode(0o007)
		require.NotEqual(t, mode, info.Mode())
		require.NoError(t, c.Chmod(filePath, mode))
		require.Equal(t, mode, info.Mode())

		_, err = c.Stat("/notanother/path/to/file.txt")
		checkErrorPath(t, err, "/notanother/path/to/file.txt")

		checkErrorPath(t, c.Chtimes("/notanother/path/to/file.txt", time.Now(), time.Now()), "/notanother/path/to/file.txt")

		checkErrorPath(t, c.Chmod("/notanother/path/to/file.txt", mode), "/notanother/path/to/file.txt")
	})

	t.Run("LstatIfPossible", func(t *testing.T) {
		t.Parallel()
		c := newTestFS()
		info, ok, err := c.LstatIfPossible(filePath)
		require.NoError(t, err)
		require.False(t, ok)
		require.Equal(t, "file.txt", info.Name())

		_, _, err = c.LstatIfPossible("/notanother/path/to/file.txt")
		checkErrorPath(t, err, "/notanother/path/to/file.txt")
	})

	t.Run("Rename", func(t *testing.T) {
		t.Parallel()
		c := newTestFS()
		info, err := c.Stat(filePath)
		require.NoError(t, err)
		require.False(t, info.IsDir())

		require.NoError(t, c.Rename(filePath, "/another/path/to/file.doc"))

		_, err = c.Stat(filePath)
		require.Error(t, err)
		require.True(t, errors.Is(err, fs.ErrNotExist))

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
		t.Parallel()
		c := newTestFS()
		removeFilePath := "/another/file/to/remove.txt"
		_, err := c.Create(removeFilePath)
		require.NoError(t, err)

		require.NoError(t, c.Remove(removeFilePath))

		_, err = c.Stat(removeFilePath)
		require.Error(t, err)
		require.True(t, errors.Is(err, fs.ErrNotExist))

		_, err = c.Create(removeFilePath)
		require.NoError(t, err)

		require.NoError(t, c.RemoveAll(path.Dir(removeFilePath)))

		_, err = c.Stat(removeFilePath)
		require.Error(t, err)
		require.True(t, errors.Is(err, fs.ErrNotExist))

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
	var perr *fs.PathError
	require.True(t, errors.As(err, &perr))
	require.Equal(t, perr.Path, path)
}
