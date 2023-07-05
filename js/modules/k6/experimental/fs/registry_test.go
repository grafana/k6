package fs

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestFileRegistryOpen(t *testing.T) {
	t.Parallel()

	t.Run("open succeeds", func(t *testing.T) {
		t.Parallel()

		registry := &registry{}
		fs := newTestFs(t, func(fs afero.Fs) error {
			return afero.WriteFile(fs, "bonjour.txt", []byte("Bonjour, le monde"), 0o644)
		})

		_, gotBeforeOk := registry.files.Load("bonjour.txt")
		gotData, gotErr := registry.open("bonjour.txt", fs)
		_, gotAfterOk := registry.files.Load("bonjour.txt")

		assert.False(t, gotBeforeOk)
		assert.NoError(t, gotErr)
		assert.Equal(t, []byte("Bonjour, le monde"), gotData)
		assert.True(t, gotAfterOk)
	})

	t.Run("double open succeeds", func(t *testing.T) {
		t.Parallel()

		registry := &registry{}
		fs := newTestFs(t, func(fs afero.Fs) error {
			return afero.WriteFile(fs, "bonjour.txt", []byte("Bonjour, le monde"), 0o644)
		})

		firstData, firstErr := registry.open("bonjour.txt", fs)
		_, gotFirstOk := registry.files.Load("bonjour.txt")
		secondData, secondErr := registry.open("bonjour.txt", fs)
		_, gotSecondOk := registry.files.Load("bonjour.txt")

		assert.True(t, gotFirstOk)
		assert.NoError(t, firstErr)
		assert.Equal(t, []byte("Bonjour, le monde"), firstData)
		assert.True(t, gotSecondOk)
		assert.NoError(t, secondErr)
		assert.Equal(t, firstData, secondData) // same pointer
		assert.Equal(t, []byte("Bonjour, le monde"), secondData)
	})
}
