package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirMake(t *testing.T) {
	t.Parallel()

	t.Run("dir_provided", func(t *testing.T) {
		t.Parallel()

		vfs := afero.NewMemMapFs()
		dir := filepath.Join("browser", "user-data")
		require.NoError(t, vfs.MkdirAll(dir, 0o700))
		s := Dir{
			fsMkdirTemp: func(string, string) (string, error) {
				t.Error("should not create a directory when one is provided")
				return "", errors.New("unexpected directory creation")
			},
			fsRemoveAll: func(string) error {
				t.Error("should not remove a provided directory")
				return errors.New("unexpected directory removal")
			},
		}
		require.NoError(t, s.Make("", dir))
		require.Equal(t, dir, s.Dir, "should return the directory")
		require.NoError(t, s.Cleanup())
		exists, err := afero.DirExists(vfs, dir)
		require.NoError(t, err)
		assert.True(t, exists, "should not remove directory")
	})

	t.Run("dir_absent", func(t *testing.T) {
		t.Parallel()

		vfs := afero.NewMemMapFs()
		tmpDir := "browser-temp"
		require.NoError(t, vfs.MkdirAll(tmpDir, 0o700))
		s := Dir{
			fsMkdirTemp: func(dir, pattern string) (string, error) {
				return afero.TempDir(vfs, dir, strings.TrimSuffix(pattern, "*"))
			},
			fsRemoveAll: vfs.RemoveAll,
		}
		require.NoError(t, s.Make(tmpDir, ""))
		assert.Equal(t, tmpDir, filepath.Dir(s.Dir))
		assert.True(t, strings.HasPrefix(filepath.Base(s.Dir), "k6browser-data-"))
		exists, err := afero.DirExists(vfs, s.Dir)
		require.NoError(t, err)
		require.True(t, exists)
		// Include content so cleanup must remove the directory recursively.
		require.NoError(t, afero.WriteFile(vfs, filepath.Join(s.Dir, "profile"), []byte("data"), 0o600))

		require.NoError(t, s.Cleanup())
		_, err = vfs.Stat(s.Dir)
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("dir_mk_err", func(t *testing.T) {
		t.Parallel()

		tmpDir := "browser-temp"
		failedPath := filepath.Join(tmpDir, "failed-directory")
		for _, tc := range []struct {
			name string
			err  error
			path string
		}{
			{
				name: "path_error",
				err:  &fs.PathError{Op: "mkdir", Path: failedPath, Err: fs.ErrPermission},
				path: failedPath,
			},
			{
				name: "ordinary_error",
				err:  fs.ErrPermission,
				path: filepath.Join(tmpDir, K6BrowserDataDirPattern),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				s := Dir{
					fsMkdirTemp: func(string, string) (string, error) {
						return "", tc.err
					},
					fsRemoveAll: func(string) error {
						t.Error("should not remove a directory after failed creation")
						return errors.New("unexpected directory removal")
					},
				}
				err := s.Make(tmpDir, "")
				require.ErrorIs(t, err, fs.ErrPermission)
				assert.EqualError(t, err, fmt.Sprintf("making browser data directory %q: %s", tc.path, fs.ErrPermission))
				assert.Empty(t, s.Dir)
				require.NoError(t, s.Cleanup())
			})
		}
	})
}

func TestDirCleanup(t *testing.T) {
	t.Parallel()

	t.Run("before_make", func(t *testing.T) {
		t.Parallel()

		s := Dir{
			fsRemoveAll: func(string) error {
				t.Error("should not remove a directory before creation")
				return errors.New("unexpected directory removal")
			},
		}
		require.NoError(t, s.Cleanup())
	})

	t.Run("remove_error", func(t *testing.T) {
		t.Parallel()

		vfs := afero.NewMemMapFs()
		tmpDir := "browser-temp"
		require.NoError(t, vfs.MkdirAll(tmpDir, 0o700))
		s := Dir{
			fsMkdirTemp: func(dir, pattern string) (string, error) {
				return afero.TempDir(vfs, dir, strings.TrimSuffix(pattern, "*"))
			},
			fsRemoveAll: func(path string) error {
				return &fs.PathError{Op: "remove", Path: path, Err: fs.ErrPermission}
			},
		}
		require.NoError(t, s.Make(tmpDir, ""))
		err := s.Cleanup()
		require.ErrorIs(t, err, fs.ErrPermission)
		var pathErr *fs.PathError
		require.ErrorAs(t, err, &pathErr)
		assert.Equal(t, s.Dir, pathErr.Path)
		exists, err := afero.DirExists(vfs, s.Dir)
		require.NoError(t, err)
		assert.True(t, exists, "failed cleanup should leave the directory intact")
	})
}
