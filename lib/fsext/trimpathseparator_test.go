package fsext

import (
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestTrimAferoPathSeparatorFs(t *testing.T) {
	m := afero.NewMemMapFs()
	fs := NewTrimFilePathSeparatorFs(m)
	expecteData := []byte("something")
	err := afero.WriteFile(fs, "/path/to/somewhere", expecteData, 0644)
	require.NoError(t, err)
	data, err := afero.ReadFile(m, "/path/to/somewhere")
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	require.Nil(t, data)

	data, err = afero.ReadFile(m, "path/to/somewhere")
	require.NoError(t, err)
	require.Equal(t, expecteData, data)
}
