package fsext

import (
	"io"
	"io/fs"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBridgeOpen(t *testing.T) {
	t.Parallel()

	testfs := afero.NewMemMapFs()
	require.NoError(t, WriteFile(testfs, "abasicpath/onetwo.txt", []byte(`test123`), 0o644))

	bridge := &IOFSBridge{FSExt: testfs}

	// it asserts that bridge implements io/fs.FS
	goiofs := fs.FS(bridge)
	f, err := goiofs.Open("abasicpath/onetwo.txt")
	require.NoError(t, err)
	require.NotNil(t, f)

	content, err := io.ReadAll(f)
	require.NoError(t, err)

	assert.Equal(t, "test123", string(content))
}
