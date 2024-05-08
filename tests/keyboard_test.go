package tests

import (
	_ "embed"
	"runtime"
	"testing"

	"github.com/grafana/xk6-browser/keyboardlayout"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyboardPress(t *testing.T) {
	t.Parallel()

	t.Run("all_keys", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()
		layout := keyboardlayout.GetKeyboardLayout("us")

		assert.NotPanics(t, func() {
			for k := range layout.Keys {
				require.NoError(t, kb.Press(string(k), nil))
			}
		})
	})

	t.Run("backspace", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		p.SetContent(`<input>`, nil)
		el, err := p.Query("input")
		require.NoError(t, err)
		p.Focus("input", nil)

		kb.Type("Hello World!", nil)
		require.Equal(t, "Hello World!", el.InputValue(nil))

		require.NoError(t, kb.Press("Backspace", nil))
		assert.Equal(t, "Hello World", el.InputValue(nil))
	})

	t.Run("combo", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		p.SetContent(`<input>`, nil)
		el, err := p.Query("input")
		require.NoError(t, err)
		p.Focus("input", nil)

		require.NoError(t, kb.Press("Shift++", nil))
		require.NoError(t, kb.Press("Shift+=", nil))
		require.NoError(t, kb.Press("Shift+@", nil))
		require.NoError(t, kb.Press("Shift+6", nil))
		require.NoError(t, kb.Press("Shift+KeyA", nil))
		require.NoError(t, kb.Press("Shift+b", nil))
		require.NoError(t, kb.Press("Shift+C", nil))

		require.NoError(t, kb.Press("Control+KeyI", nil))
		require.NoError(t, kb.Press("Control+J", nil))
		require.NoError(t, kb.Press("Control+k", nil))

		require.Equal(t, "+=@6AbC", el.InputValue(nil))
	})

	t.Run("meta", func(t *testing.T) {
		t.Parallel()
		t.Skip("FIXME") // See https://github.com/grafana/xk6-browser/issues/424
		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		p.SetContent(`<input>`, nil)
		el, err := p.Query("input")
		require.NoError(t, err)
		p.Focus("input", nil)

		require.NoError(t, kb.Press("Shift+KeyA", nil))
		require.NoError(t, kb.Press("Shift+b", nil))
		require.NoError(t, kb.Press("Shift+C", nil))

		require.Equal(t, "AbC", el.InputValue(nil))

		metaKey := "Control"
		if runtime.GOOS == "darwin" {
			metaKey = "Meta"
		}
		require.NoError(t, kb.Press(metaKey+"+A", nil))
		require.NoError(t, kb.Press("Delete", nil))
		assert.Equal(t, "", el.InputValue(nil))
	})

	t.Run("type does not split on +", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		p.SetContent(`<textarea>`, nil)
		el, err := p.Query("textarea")
		require.NoError(t, err)
		p.Focus("textarea", nil)

		kb.Type("L+m+KeyN", nil)
		assert.Equal(t, "L+m+KeyN", el.InputValue(nil))
	})

	t.Run("capitalization", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		p.SetContent(`<textarea>`, nil)
		el, err := p.Query("textarea")
		require.NoError(t, err)
		p.Focus("textarea", nil)

		require.NoError(t, kb.Press("C", nil))
		require.NoError(t, kb.Press("d", nil))
		require.NoError(t, kb.Press("KeyE", nil))

		require.NoError(t, kb.Down("Shift"))
		require.NoError(t, kb.Down("f"))
		require.NoError(t, kb.Up("f"))
		require.NoError(t, kb.Down("G"))
		require.NoError(t, kb.Up("G"))
		require.NoError(t, kb.Down("KeyH"))
		require.NoError(t, kb.Up("KeyH"))
		require.NoError(t, kb.Up("Shift"))

		assert.Equal(t, "CdefGH", el.InputValue(nil))
	})

	t.Run("type not affected by shift", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		p.SetContent(`<textarea>`, nil)
		el, err := p.Query("textarea")
		require.NoError(t, err)
		p.Focus("textarea", nil)

		require.NoError(t, kb.Down("Shift"))
		kb.Type("oPqR", nil)
		require.NoError(t, kb.Up("Shift"))

		assert.Equal(t, "oPqR", el.InputValue(nil))
	})

	t.Run("newline", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		p.SetContent(`<textarea>`, nil)
		el, err := p.Query("textarea")
		require.NoError(t, err)
		p.Focus("textarea", nil)

		kb.Type("Hello", nil)
		require.NoError(t, kb.Press("Enter", nil))
		require.NoError(t, kb.Press("Enter", nil))
		kb.Type("World!", nil)
		assert.Equal(t, "Hello\n\nWorld!", el.InputValue(nil))
	})

	// Replicates the test from https://playwright.dev/docs/api/class-keyboard
	t.Run("selection", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		p.SetContent(`<input>`, nil)
		el, err := p.Query("input")
		require.NoError(t, err)
		p.Focus("input", nil)

		kb.Type("Hello World!", nil)
		require.Equal(t, "Hello World!", el.InputValue(nil))

		require.NoError(t, kb.Press("ArrowLeft", nil))
		// Should hold the key until Up() is called.
		require.NoError(t, kb.Down("Shift"))
		for i := 0; i < len(" World"); i++ {
			require.NoError(t, kb.Press("ArrowLeft", nil))
		}
		// Should release the key but the selection should remain active.
		kb.Up("Shift")
		// Should delete the selection.
		require.NoError(t, kb.Press("Backspace", nil))

		assert.Equal(t, "Hello!", el.InputValue(nil))
	})
}
