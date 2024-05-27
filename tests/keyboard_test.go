package tests

import (
	_ "embed"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/keyboardlayout"
)

func TestKeyboardPress(t *testing.T) {
	t.Parallel()

	t.Run("all_keys", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()
		layout := keyboardlayout.GetKeyboardLayout("us")

		for k := range layout.Keys {
			assert.NoError(t, kb.Press(string(k), nil))
		}
	})

	t.Run("backspace", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		err := p.SetContent(`<input>`, nil)
		require.NoError(t, err)
		el, err := p.Query("input")
		require.NoError(t, err)
		require.NoError(t, p.Focus("input", nil))

		require.NoError(t, kb.Type("Hello World!", nil))
		v, err := el.InputValue(nil)
		require.NoError(t, err)
		require.Equal(t, "Hello World!", v)

		require.NoError(t, kb.Press("Backspace", nil))
		v, err = el.InputValue(nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello World", v)
	})

	t.Run("combo", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		err := p.SetContent(`<input>`, nil)
		require.NoError(t, err)
		el, err := p.Query("input")
		require.NoError(t, err)
		require.NoError(t, p.Focus("input", nil))

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

		v, err := el.InputValue(nil)
		require.NoError(t, err)
		require.Equal(t, "+=@6AbC", v)
	})

	t.Run("meta", func(t *testing.T) {
		t.Parallel()
		t.Skip("FIXME") // See https://github.com/grafana/xk6-browser/issues/424
		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		err := p.SetContent(`<input>`, nil)
		require.NoError(t, err)
		el, err := p.Query("input")
		require.NoError(t, err)
		require.NoError(t, p.Focus("input", nil))

		require.NoError(t, kb.Press("Shift+KeyA", nil))
		require.NoError(t, kb.Press("Shift+b", nil))
		require.NoError(t, kb.Press("Shift+C", nil))

		v, err := el.InputValue(nil)
		require.NoError(t, err)
		require.Equal(t, "AbC", v)

		metaKey := "Control"
		if runtime.GOOS == "darwin" {
			metaKey = "Meta"
		}
		require.NoError(t, kb.Press(metaKey+"+A", nil))
		require.NoError(t, kb.Press("Delete", nil))
		v, err = el.InputValue(nil)
		require.NoError(t, err)
		assert.Equal(t, "", v)
	})

	t.Run("type does not split on +", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		err := p.SetContent(`<textarea>`, nil)
		require.NoError(t, err)
		el, err := p.Query("textarea")
		require.NoError(t, err)
		require.NoError(t, p.Focus("textarea", nil))

		require.NoError(t, kb.Type("L+m+KeyN", nil))
		v, err := el.InputValue(nil)
		require.NoError(t, err)
		assert.Equal(t, "L+m+KeyN", v)
	})

	t.Run("capitalization", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		err := p.SetContent(`<textarea>`, nil)
		require.NoError(t, err)
		el, err := p.Query("textarea")
		require.NoError(t, err)
		require.NoError(t, p.Focus("textarea", nil))

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

		v, err := el.InputValue(nil)
		require.NoError(t, err)
		assert.Equal(t, "CdefGH", v)
	})

	t.Run("type not affected by shift", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		err := p.SetContent(`<textarea>`, nil)
		require.NoError(t, err)
		el, err := p.Query("textarea")
		require.NoError(t, err)
		require.NoError(t, p.Focus("textarea", nil))

		require.NoError(t, kb.Down("Shift"))
		require.NoError(t, kb.Type("oPqR", nil))
		require.NoError(t, kb.Up("Shift"))

		v, err := el.InputValue(nil)
		require.NoError(t, err)
		assert.Equal(t, "oPqR", v)
	})

	t.Run("newline", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		err := p.SetContent(`<textarea>`, nil)
		require.NoError(t, err)
		el, err := p.Query("textarea")
		require.NoError(t, err)
		require.NoError(t, p.Focus("textarea", nil))

		require.NoError(t, kb.Type("Hello", nil))
		require.NoError(t, kb.Press("Enter", nil))
		require.NoError(t, kb.Press("Enter", nil))
		require.NoError(t, kb.Type("World!", nil))
		v, err := el.InputValue(nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello\n\nWorld!", v)
	})

	// Replicates the test from https://playwright.dev/docs/api/class-keyboard
	t.Run("selection", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		err := p.SetContent(`<input>`, nil)
		require.NoError(t, err)
		el, err := p.Query("input")
		require.NoError(t, err)
		require.NoError(t, p.Focus("input", nil))

		require.NoError(t, kb.Type("Hello World!", nil))
		v, err := el.InputValue(nil)
		require.NoError(t, err)
		require.Equal(t, "Hello World!", v)

		require.NoError(t, kb.Press("ArrowLeft", nil))
		// Should hold the key until Up() is called.
		require.NoError(t, kb.Down("Shift"))
		for i := 0; i < len(" World"); i++ {
			require.NoError(t, kb.Press("ArrowLeft", nil))
		}
		// Should release the key but the selection should remain active.
		require.NoError(t, kb.Up("Shift"))
		// Should delete the selection.
		require.NoError(t, kb.Press("Backspace", nil))

		require.NoError(t, err)
		require.NoError(t, err)
		assert.Equal(t, "Hello World!", v)
	})
}
