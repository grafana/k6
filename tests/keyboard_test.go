package tests

import (
	_ "embed"
	"runtime"
	"testing"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/keyboardlayout"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyboardPress(t *testing.T) {
	tb := newTestBrowser(t)

	t.Run("all_keys", func(t *testing.T) {
		p := tb.NewPage(nil)
		cp, ok := p.(*common.Page)
		require.True(t, ok)
		kb := cp.Keyboard
		layout := keyboardlayout.GetKeyboardLayout("us")

		assert.NotPanics(t, func() {
			for k := range layout.Keys {
				kb.Press(string(k), nil)
			}
		})
	})

	t.Run("backspace", func(t *testing.T) {
		p := tb.NewPage(nil)
		cp, ok := p.(*common.Page)
		require.True(t, ok)
		kb := cp.Keyboard

		p.SetContent(`<input>`, nil)
		el := p.Query("input")
		p.Focus("input", nil)

		kb.Type("Hello World!", nil)
		require.Equal(t, "Hello World!", el.InputValue(nil))

		kb.Press("Backspace", nil)
		assert.Equal(t, "Hello World", el.InputValue(nil))
	})

	t.Run("combo", func(t *testing.T) {
		t.Skip("FIXME") // See https://github.com/grafana/xk6-browser/issues/285
		p := tb.NewPage(nil)
		cp, ok := p.(*common.Page)
		require.True(t, ok)
		kb := cp.Keyboard

		p.SetContent(`<input>`, nil)
		el := p.Query("input")
		p.Focus("input", nil)

		kb.Press("Shift+KeyA", nil)
		kb.Press("Shift+b", nil)
		kb.Press("C", nil)
		kb.Press("d", nil)
		require.Equal(t, "AbCd", el.InputValue(nil))

		metaKey := "Control"
		if runtime.GOOS == "darwin" {
			metaKey = "Meta"
		}
		kb.Press(metaKey+"+A", nil)
		kb.Press("Delete", nil)
		assert.Equal(t, "", el.InputValue(nil))
	})

	t.Run("newline", func(t *testing.T) {
		p := tb.NewPage(nil)
		cp, ok := p.(*common.Page)
		require.True(t, ok)
		kb := cp.Keyboard

		p.SetContent(`<textarea>`, nil)
		el := p.Query("textarea")
		p.Focus("textarea", nil)

		kb.Type("Hello", nil)
		kb.Press("Enter", nil)
		kb.Press("Enter", nil)
		kb.Type("World!", nil)
		assert.Equal(t, "Hello\n\nWorld!", el.InputValue(nil))
	})

	// Replicates the test from https://playwright.dev/docs/api/class-keyboard
	t.Run("selection", func(t *testing.T) {
		t.Skip("FIXME") // See https://github.com/grafana/xk6-browser/issues/284
		p := tb.NewPage(nil)
		cp, ok := p.(*common.Page)
		require.True(t, ok)
		kb := cp.Keyboard

		p.SetContent(`<input>`, nil)
		el := p.Query("input")
		p.Focus("input", nil)

		kb.Type("Hello World!", nil)
		require.Equal(t, "Hello World!", el.InputValue(nil))

		kb.Press("ArrowLeft", nil)
		// Should hold the key until Up() is called.
		kb.Down("Shift")
		for i := 0; i < len(" World"); i++ {
			kb.Press("ArrowLeft", nil)
		}
		// Should release the key but the selection should remain active.
		kb.Up("Shift")
		// Should delete the selection.
		kb.Press("Backspace", nil)

		assert.Equal(t, "Hello!", el.InputValue(nil))
	})
}
