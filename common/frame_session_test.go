package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// See: Issue #183 for details.
func TestFrameSessionAddInsetToViewport(t *testing.T) {
	t.Parallel()

	// shouldn't change the viewport if headless is true.
	t.Run("headless_true", func(t *testing.T) {
		t.Parallel()

		headless, vp := true, Viewport{}
		addInsetToViewport(&vp, headless, "any os")
		assert.Equal(t, vp, Viewport{})
	})

	// should add the default inset to viewport if the
	// operating system is unrecognized.
	t.Run("headless_false", func(t *testing.T) {
		t.Parallel()

		var (
			headless bool
			vp       Viewport
		)
		addInsetToViewport(&vp, headless, "any os")
		assert.NotEqual(t, vp, Viewport{})
	})

	// should add a different inset to viewport than the default one
	// if a recognized os is given.
	for _, os := range []string{"windows", "linux", "darwin"} {
		os := os
		t.Run(os, func(t *testing.T) {
			t.Parallel()

			var (
				headless bool
				vp       Viewport
			)
			// add the default inset to the viewport
			addInsetToViewport(&vp, headless, "any os")
			defaultVp := vp
			// add os specific inset to the viewport
			addInsetToViewport(&vp, headless, os)

			assert.NotEqual(t, vp, defaultVp, "inset for %q should exist", os)
			assert.NotEqual(t, vp, Viewport{
				Width:  defaultVp.Width * 2,
				Height: defaultVp.Height * 2,
			}, "inset for %q should not be the same as the default one", os)
		})
	}
}
