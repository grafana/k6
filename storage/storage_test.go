package storage

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirMake(t *testing.T) {
	t.Parallel()

	tmpDir := os.TempDir() //nolint:forbidigo

	t.Run("dir_provided", func(t *testing.T) {
		t.Parallel()

		dir, err := os.MkdirTemp("", "*") //nolint:forbidigo
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) }) //nolint:forbidigo

		var s Dir
		require.NoError(t, s.Make("", dir))
		require.Equal(t, dir, s.Dir, "should return the directory")
		assert.NotPanics(t, func() {
			t.Helper()
			err := s.Cleanup()
			assert.NoError(t, err)
		}) // should be a no-op
		assert.DirExists(t, dir, "should not remove directory")
	})

	t.Run("dir_absent", func(t *testing.T) {
		t.Parallel()

		var s Dir
		require.NoError(t, s.Make("", ""))
		require.True(t, strings.HasPrefix(s.Dir, tmpDir))
		require.DirExists(t, s.Dir)

		assert.NotPanics(t, func() {
			t.Helper()
			err := s.Cleanup()
			assert.NoError(t, err)
		})
		require.NoDirExists(t, s.Dir)
	})

	t.Run("dir_mk_err", func(t *testing.T) {
		t.Parallel()

		var s Dir
		require.ErrorIs(t, s.Make("/NOT_EXISTING_DIRECTORY/K6/BROWSER", ""), fs.ErrNotExist)
		assert.Empty(t, s.Dir)

		assert.NotPanics(t, func() {
			t.Helper()
			err := s.Cleanup()
			assert.NoError(t, err)
		})
	})
}
