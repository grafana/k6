package fsext

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestTrimAferoPathSeparatorFs(t *testing.T) {
	t.Parallel()
	m := afero.NewMemMapFs()
	f := NewTrimFilePathSeparatorFs(m)
	expecteData := []byte("something")
	err := afero.WriteFile(f, filepath.FromSlash("/path/to/somewhere"), expecteData, 0o644)
	require.NoError(t, err)
	data, err := afero.ReadFile(m, "/path/to/somewhere")
	require.Error(t, err)
	require.True(t, errors.Is(err, fs.ErrNotExist))
	require.Nil(t, data)

	data, err = afero.ReadFile(m, "path/to/somewhere")
	require.NoError(t, err)
	require.Equal(t, expecteData, data)

	err = afero.WriteFile(f, filepath.FromSlash("path/without/separtor"), expecteData, 0o644)
	require.Error(t, err)
	require.True(t, errors.Is(err, fs.ErrNotExist))
}
