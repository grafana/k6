package testutils

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/fsext"
)

// MakeMemMapFs creates a new in-memory filesystem with the given files.
//
// It is particularly useful for testing code that interacts with the
// filesystem, as it  allows to create a filesystem with a known state
// without having to create temporary directories and files on disk.
//
// The keys of the withFiles map are the paths of the files to create, and the
// values are the contents of the files. The files are created with 644 mode.
//
// The filesystem is returned as a [fsext.Fs].
func MakeMemMapFs(t *testing.T, withFiles map[string][]byte) fsext.Fs {
	fs := fsext.NewMemMapFs()

	for path, data := range withFiles {
		require.NoError(t, fsext.WriteFile(fs, path, data, 0o644))
	}

	return fs
}
