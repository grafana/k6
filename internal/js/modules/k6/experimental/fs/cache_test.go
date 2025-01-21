package fs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/lib/fsext"
)

func TestFileCacheOpen(t *testing.T) {
	t.Parallel()

	t.Run("open succeeds", func(t *testing.T) {
		t.Parallel()

		cache := &cache{}
		fs := newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, "bonjour.txt", []byte("Bonjour, le monde"), 0o644)
		})

		_, gotBeforeOk := cache.openedFiles.Load("bonjour.txt")
		gotData, gotErr := cache.open("bonjour.txt", fs)
		_, gotAfterOk := cache.openedFiles.Load("bonjour.txt")

		assert.False(t, gotBeforeOk)
		assert.NoError(t, gotErr)
		assert.Equal(t, []byte("Bonjour, le monde"), gotData)
		assert.True(t, gotAfterOk)
	})

	t.Run("double open succeeds", func(t *testing.T) {
		t.Parallel()

		cache := &cache{}
		fs := newTestFs(t, func(fs fsext.Fs) error {
			return fsext.WriteFile(fs, "bonjour.txt", []byte("Bonjour, le monde"), 0o644)
		})

		firstData, firstErr := cache.open("bonjour.txt", fs)
		_, gotFirstOk := cache.openedFiles.Load("bonjour.txt")
		secondData, secondErr := cache.open("bonjour.txt", fs)
		_, gotSecondOk := cache.openedFiles.Load("bonjour.txt")

		assert.True(t, gotFirstOk)
		assert.NoError(t, firstErr)
		assert.Equal(t, []byte("Bonjour, le monde"), firstData)
		assert.True(t, gotSecondOk)
		assert.NoError(t, secondErr)
		assert.True(t, sameUnderlyingArray(firstData, secondData))
		assert.Equal(t, []byte("Bonjour, le monde"), secondData)
	})
}

// sameUnderlyingArray returns true if the underlying array of lhs and rhs are the same.
//
// This is done by checking that the two slices have a capacity greater than 0 and that
// the last element of the underlying array is the same for both slices.
//
// Once a slice is created, its starting address can move forward, but can never move
// behond its starting address + its capacity, which is a fixed value for any Go slice.
//
// Hence, if the last element of the underlying array is the same for both slices, it
// means that the underlying array is the same.
//
// See [explanation] for more details.
//
// [explanation]: https://groups.google.com/g/golang-nuts/c/ks1jvoyMYuc?pli=1
func sameUnderlyingArray(lhs, rhs []byte) bool {
	return cap(lhs) > 0 && cap(rhs) > 0 && &lhs[0:cap(lhs)][cap(lhs)-1] == &rhs[0:cap(rhs)][cap(rhs)-1]
}
