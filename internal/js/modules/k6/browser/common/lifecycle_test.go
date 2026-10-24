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

func TestLifecycleEventMarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("load", func(t *testing.T) {
		t.Parallel()

		evt := LifecycleEventLoad
		m, err := evt.MarshalJSON()
		require.NoError(t, err)
		assert.Equal(t, []byte(`"load"`), m)
	})

	t.Run("domcontentloaded", func(t *testing.T) {
		t.Parallel()

		evt := LifecycleEventDOMContentLoad
		m, err := evt.MarshalJSON()
		require.NoError(t, err)
		assert.Equal(t, []byte(`"domcontentloaded"`), m)
	})

	t.Run("networkidle", func(t *testing.T) {
		t.Parallel()

		evt := LifecycleEventNetworkIdle
		m, err := evt.MarshalJSON()
		require.NoError(t, err)
		assert.Equal(t, []byte(`"networkidle"`), m)
	})
}

func TestLifecycleEventUnmarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("load", func(t *testing.T) {
		t.Parallel()

		var evt LifecycleEvent
		err := evt.UnmarshalJSON([]byte(`"load"`))
		require.NoError(t, err)
		assert.Equal(t, LifecycleEventLoad, evt)
	})

	t.Run("domcontentloaded", func(t *testing.T) {
		t.Parallel()

		var evt LifecycleEvent
		err := evt.UnmarshalJSON([]byte(`"domcontentloaded"`))
		require.NoError(t, err)
		assert.Equal(t, LifecycleEventDOMContentLoad, evt)
	})

	t.Run("networkidle", func(t *testing.T) {
		t.Parallel()

		var evt LifecycleEvent
		err := evt.UnmarshalJSON([]byte(`"networkidle"`))
		require.NoError(t, err)
		assert.Equal(t, LifecycleEventNetworkIdle, evt)
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		var evt LifecycleEvent
		err := evt.UnmarshalJSON([]byte(`load`))

		require.Error(t, err)
	})
}
