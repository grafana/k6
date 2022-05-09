package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// See: Issue #183 for details.
func TestViewportCalculateInset(t *testing.T) {
	t.Parallel()

	t.Run("headless", func(t *testing.T) {
		t.Parallel()

		headless, vp := true, Viewport{}
		vp.calculateInset(headless, "any os")
		assert.Equal(t, vp, Viewport{},
			"should not change the viewport if headless is true")
	})

	t.Run("headful", func(t *testing.T) {
		t.Parallel()

		var (
			headless bool
			vp       Viewport
		)
		vp.calculateInset(headless, "any os")
		assert.NotEqual(t, vp, Viewport{},
			"should add the default inset to the viewport if the"+
				" operating system is unrecognized by the addInset.")
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
			vp.calculateInset(headless, "any os")
			defaultVp := vp
			// add an os specific inset to the viewport
			vp.calculateInset(headless, os)

			assert.NotEqual(t, vp, defaultVp, "inset for %q should exist", os)
			// we multiply the default viewport by two to detect
			// whether an os specific inset is adding the default
			// viewport, instead of its own.
			assert.NotEqual(t, vp, Viewport{
				Width:  defaultVp.Width * 2,
				Height: defaultVp.Height * 2,
			}, "inset for %q should not be the same as the default one", os)
		})
	}
}

func TestLifecycleEventMarshalText(t *testing.T) {
	t.Parallel()

	t.Run("ok/nil", func(t *testing.T) {
		t.Parallel()

		var evt *LifecycleEvent
		m, err := evt.MarshalText()
		require.NoError(t, err)
		assert.Equal(t, []byte(""), m)
	})

	t.Run("err/invalid", func(t *testing.T) {
		t.Parallel()

		evt := LifecycleEvent(-1)
		_, err := evt.MarshalText()
		require.EqualError(t, err, "invalid lifecycle event: -1")
	})
}

func TestLifecycleEventMarshalTextRound(t *testing.T) {
	t.Parallel()

	evt := LifecycleEventLoad
	m, err := evt.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, []byte("load"), m)

	var evt2 LifecycleEvent
	err = evt2.UnmarshalText(m)
	require.NoError(t, err)
	assert.Equal(t, evt, evt2)
}

func TestLifecycleEventUnmarshalText(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		var evt LifecycleEvent
		err := evt.UnmarshalText([]byte("load"))
		require.NoError(t, err)
		assert.Equal(t, LifecycleEventLoad, evt)
	})

	t.Run("err/invalid", func(t *testing.T) {
		t.Parallel()

		var evt LifecycleEvent
		err := evt.UnmarshalText([]byte("none"))
		require.EqualError(t, err,
			`invalid lifecycle event: "none"; `+
				`must be one of: load, domcontentloaded, networkidle`)
	})

	t.Run("err/invalid_empty", func(t *testing.T) {
		t.Parallel()

		var evt LifecycleEvent
		err := evt.UnmarshalText([]byte(""))
		require.EqualError(t, err,
			`invalid lifecycle event: ""; `+
				`must be one of: load, domcontentloaded, networkidle`)
	})
}
