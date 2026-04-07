package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// See: Issue #183 for details.
func TestViewportCalculateInset(t *testing.T) {
	t.Parallel()

	t.Run("headless", func(t *testing.T) {
		t.Parallel()

		headless, vp := true, Viewport{}
		vp = vp.recalculateInset(headless, "any os")
		assert.Equal(t, Viewport{}, vp,
			"should not change the viewport if headless is true")
	})

	t.Run("headful", func(t *testing.T) {
		t.Parallel()

		var (
			headless bool
			vp       Viewport
		)
		vp = vp.recalculateInset(headless, "any os")
		assert.NotEqual(t, Viewport{}, vp,
			"should add the default inset to the viewport if the"+
				" operating system is unrecognized by the addInset.")
	})

	// should add a different inset to viewport than the default one
	// if a recognized os is given.
	for _, os := range []string{"windows", "linux", "darwin"} {
		t.Run(os, func(t *testing.T) {
			t.Parallel()

			var (
				headless bool
				vp       Viewport
			)
			// add the default inset to the viewport
			vp = vp.recalculateInset(headless, "any os")
			defaultVp := vp
			// add an os specific inset to the viewport
			vp = vp.recalculateInset(headless, os)

			assert.NotEqual(t, defaultVp, vp, "inset for %q should exist", os)
			// we multiply the default viewport by two to detect
			// whether an os specific inset is adding the default
			// viewport, instead of its own.
			assert.NotEqual(t, Viewport{
				Width:  defaultVp.Width * 2,
				Height: defaultVp.Height * 2,
			}, vp, "inset for %q should not be the same as the default one", os)
		})
	}
}
