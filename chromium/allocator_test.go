package chromium

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeUserDataDir(t *testing.T) {
	tmpDir := os.TempDir()

	t.Run("user_dir_provided", func(t *testing.T) {
		usrDir, err := os.MkdirTemp("", "*")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(usrDir) })

		dir, remove, err := makeUserDataDir(tmpDir, usrDir)

		require.NoError(t, err)
		require.Equal(t, usrDir, dir, "should return the user's directory")
		assert.NotPanics(t, remove) // should be a no-op
		assert.DirExists(t, usrDir, "should not remove user's directory")
	})

	t.Run("user_dir_absent", func(t *testing.T) {
		const usrDir = ""

		dir, remove, err := makeUserDataDir(tmpDir, usrDir)

		require.NoError(t, err)
		require.True(t, strings.HasPrefix(dir, tmpDir))
		require.DirExists(t, dir)
		assert.NotPanics(t, remove)
		require.NoDirExists(t, dir)
	})

	t.Run("user_dir_mk_err", func(t *testing.T) {
		dir, remove, err := makeUserDataDir("/NOT_EXISTING_DIRECTORY/K6/BROWSER", "")

		require.ErrorIs(t, err, fs.ErrNotExist)
		assert.Empty(t, dir)
		assert.NotPanics(t, remove)
	})
}
