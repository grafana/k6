package tests

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/keyboardlayout"
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
}
