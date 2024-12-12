package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
