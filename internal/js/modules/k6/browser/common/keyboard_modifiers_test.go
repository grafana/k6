package common

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyboardHoldModifiers(t *testing.T) {
	t.Parallel()

	newKB := func(t *testing.T) (*Keyboard, *fakeSession) {
		t.Helper()
		sess := &fakeSession{
			session: &Session{id: "1234"},
		}
		return NewKeyboard(context.Background(), sess), sess
	}

	t.Run("presses and restores", func(t *testing.T) {
		t.Parallel()
		kb, sess := newKB(t)

		restore, err := kb.holdModifiers([]string{"Shift", "Alt"})
		require.NoError(t, err)
		assert.Equal(t, ModifierKeyShift|ModifierKeyAlt, kb.modifiers)
		require.GreaterOrEqual(t, len(sess.cdpCalls), 2)

		restore()
		assert.Equal(t, int64(0), kb.modifiers)
	})

	t.Run("skips already held modifiers", func(t *testing.T) {
		t.Parallel()
		kb, sess := newKB(t)
		require.NoError(t, kb.Down("Shift"))
		callsAfterDown := len(sess.cdpCalls)

		restore, err := kb.holdModifiers([]string{"Shift"})
		require.NoError(t, err)
		assert.Equal(t, callsAfterDown, len(sess.cdpCalls))
		assert.Equal(t, ModifierKeyShift, kb.modifiers)

		restore()
		assert.Equal(t, ModifierKeyShift, kb.modifiers)
		require.NoError(t, kb.Up("Shift"))
	})

	t.Run("control or meta", func(t *testing.T) {
		t.Parallel()
		kb, _ := newKB(t)

		restore, err := kb.holdModifiers([]string{"ControlOrMeta"})
		require.NoError(t, err)
		if runtime.GOOS == "darwin" {
			assert.Equal(t, ModifierKeyMeta, kb.modifiers)
		} else {
			assert.Equal(t, ModifierKeyControl, kb.modifiers)
		}
		restore()
		assert.Equal(t, int64(0), kb.modifiers)
	})

	t.Run("unknown modifier", func(t *testing.T) {
		t.Parallel()
		kb, _ := newKB(t)

		_, err := kb.holdModifiers([]string{"NotAKey"})
		require.ErrorContains(t, err, `unknown modifier "NotAKey"`)
		assert.Equal(t, int64(0), kb.modifiers)
	})
}
