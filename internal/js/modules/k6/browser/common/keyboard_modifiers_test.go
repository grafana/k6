package common

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type keyboardModifierSession struct {
	session
	keys      []input.DispatchKeyEventParams
	failKeyAt int
	failMouse bool
	failure   error
}

func (s *keyboardModifierSession) Execute(_ context.Context, method string, params, _ any) error {
	if method == input.CommandDispatchKeyEvent {
		if event, ok := params.(*input.DispatchKeyEventParams); ok {
			s.keys = append(s.keys, *event)
		}
		if len(s.keys) == s.failKeyAt {
			return s.failure
		}
	}
	if method == input.CommandDispatchMouseEvent && s.failMouse {
		return s.failure
	}
	return nil
}

func TestKeyboardWithModifiers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		modifiers []string
		want      int64
		actionErr bool
		failKeyAt int
	}{
		{"omitted", nil, ModifierKeyShift, false, 0},
		{"empty", []string{}, 0, false, 0},
		{"replace", []string{"Alt", "Control"}, ModifierKeyAlt | ModifierKeyControl, false, 0},
		{"duplicate", []string{"Control", "Control"}, ModifierKeyControl, false, 0},
		{"action_failure", []string{"Control"}, ModifierKeyControl, true, 0},
		{"setup_failure", []string{"Control"}, ModifierKeyControl, false, 2},
		{"restore_failure", []string{"Control"}, ModifierKeyControl, false, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			failure := errors.New("input failed")
			s := &keyboardModifierSession{failure: failure, failKeyAt: tc.failKeyAt}
			keyboard := NewKeyboard(t.Context(), s)
			require.NoError(t, keyboard.Down("Shift"))
			originalKeys := maps.Clone(keyboard.pressedKeys)

			called := false
			err := keyboard.withModifiers(tc.modifiers, func() error {
				called = true
				assert.Equal(t, tc.want, keyboard.modifiers)
				if tc.actionErr {
					return failure
				}
				return nil
			})
			if tc.actionErr || tc.failKeyAt != 0 {
				require.ErrorIs(t, err, failure)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.failKeyAt != 2, called)
			assert.Equal(t, ModifierKeyShift, keyboard.modifiers)
			assert.Equal(t, originalKeys, keyboard.pressedKeys)
			if tc.modifiers == nil {
				assert.Len(t, s.keys, 1, "omitted modifiers must not send keyboard events")
			}
		})
	}
}

func TestKeyboardWithModifiersRestoresPressedKeys(t *testing.T) {
	t.Parallel()

	for name, keys := range map[string][]string{
		"right_shift":     {"ShiftRight"},
		"right_modifiers": {"AltRight", "ControlRight", "MetaRight"},
		"both_shifts":     {"ShiftLeft", "ShiftRight"},
		"ordinary_key":    {"ControlLeft", "ControlRight", "KeyA"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := &keyboardModifierSession{}
			keyboard := NewKeyboard(t.Context(), s)
			for _, key := range keys {
				require.NoError(t, keyboard.Down(key))
			}
			originalKeys := maps.Clone(keyboard.pressedKeys)
			originalModifiers := keyboard.modifiers
			require.NoError(t, keyboard.withModifiers([]string{}, func() error {
				assert.Zero(t, keyboard.modifiers)
				return nil
			}))
			assert.Equal(t, originalModifiers, keyboard.modifiers)
			assert.Equal(t, originalKeys, keyboard.pressedKeys)
		})
	}
}

func TestKeyboardWithInvalidModifier(t *testing.T) {
	t.Parallel()

	s := &keyboardModifierSession{}
	keyboard := NewKeyboard(t.Context(), s)
	err := keyboard.withModifiers([]string{"Control", "A"}, func() error {
		t.Error("action must not run with an invalid modifier")
		return nil
	})
	require.EqualError(t, err, `invalid keyboard modifier "A"`)
	assert.Empty(t, s.keys)
	assert.Zero(t, keyboard.modifiers)
}

func TestMouseClickRestoresModifiersOnFailure(t *testing.T) {
	t.Parallel()

	failure := errors.New("mouse input failed")
	s := &keyboardModifierSession{failMouse: true, failure: failure}
	keyboard := NewKeyboard(t.Context(), s)
	mouse := NewMouse(t.Context(), s, nil, NewTimeoutSettings(nil), keyboard)
	opts := NewMouseClickOptions()
	opts.modifiers = []string{"Control"}

	err := mouse.Click(10, 20, opts)
	require.ErrorIs(t, err, failure)
	assert.Zero(t, keyboard.modifiers)
	assert.Empty(t, keyboard.pressedKeys)
	require.Len(t, s.keys, 2)
	assert.Equal(t, input.KeyRawDown, s.keys[0].Type)
	assert.Equal(t, input.KeyUp, s.keys[1].Type)
}
