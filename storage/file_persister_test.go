package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalFilePersister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path         string
		existingData string
		data         string
		truncates    bool
	}{
		{
			name: "just_file",
			path: "test.txt",
			data: "some data",
		},
		{
			name: "with_dir",
			path: "path/test.txt",
			data: "some data",
		},
		{
			name:         "truncates",
			path:         "test.txt",
			data:         "some data",
			truncates:    true,
			existingData: "existing data",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			p := filepath.Join(dir, tt.path)

			// We want to make sure that the persister truncates the existing
			// data and therefore overwrites existing data. This sets up a file
			// with some existing data that should be overwritten.
			if tt.truncates {
				err := os.WriteFile(p, []byte(tt.existingData), 0o600)
				require.NoError(t, err)
			}

			var l LocalFilePersister
			err := l.Persist(p, strings.NewReader(tt.data))
			assert.NoError(t, err)

			i, err := os.Stat(p)
			require.NoError(t, err)
			assert.False(t, i.IsDir())

			bb, err := os.ReadFile(filepath.Clean(p))
			require.NoError(t, err)

			if tt.truncates {
				assert.NotEqual(t, tt.existingData, string(bb))
			}

			assert.Equal(t, tt.data, string(bb))
		})
	}
}
