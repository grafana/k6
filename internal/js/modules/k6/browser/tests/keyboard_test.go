// practically none of this work on windows
//go:build !windows

package tests

import (
	"context"
	_ "embed"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/keyboardlayout"
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
			assert.NoError(t, kb.Press(string(k), common.KeyboardOptions{}))
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
		require.NoError(t, p.Focus("input", common.NewFrameBaseOptions(p.MainFrame().Timeout())))

		require.NoError(t, kb.Type("Hello World!", common.KeyboardOptions{}))
		v, err := el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
		require.NoError(t, err)
		require.Equal(t, "Hello World!", v)

		require.NoError(t, kb.Press("Backspace", common.KeyboardOptions{}))
		v, err = el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
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
		require.NoError(t, p.Focus("input", common.NewFrameBaseOptions(p.MainFrame().Timeout())))

		require.NoError(t, kb.Press("Shift++", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Shift+=", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Shift+@", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Shift+6", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Shift+KeyA", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Shift+b", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Shift+C", common.KeyboardOptions{}))

		require.NoError(t, kb.Press("Control+KeyI", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Control+J", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Control+k", common.KeyboardOptions{}))

		v, err := el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
		require.NoError(t, err)
		require.Equal(t, "+=@6AbC", v)
	})

	t.Run("control_or_meta", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t, withFileServer())
		p := tb.NewPage(nil)

		// Navigate to page1
		url := tb.staticURL("page1.html")
		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := p.Goto(
			url,
			opts,
		)
		assert.NoError(t, err)

		// Make sure the h1 header is "Page 1"
		text, err := p.Locator("h1", nil).InnerText(nil)
		assert.NoError(t, err)
		assert.Equal(t, "Page 1", text)

		ctx, cancel := context.WithTimeout(tb.context(), 5*time.Second)
		defer cancel()

		bc := tb.Browser.Context()
		var newTab *common.Page

		// We want to meta/control click the link so that it opens in a new tab.
		// At the same time we will wait for a new page creation with WaitForEvent.
		err = tb.run(ctx,
			func() error {
				var resp any
				resp, err := bc.WaitForEvent("page", nil, 5*time.Second)
				if err != nil {
					return err
				}

				var ok bool
				newTab, ok = resp.(*common.Page)
				assert.True(t, ok)

				return nil
			},
			func() error {
				kb := p.GetKeyboard()
				assert.NoError(t, kb.Down("ControlOrMeta"))
				err = p.Locator(`a[href="page2.html"]`, nil).Click(common.NewFrameClickOptions(p.Timeout()))
				assert.NoError(t, err)
				assert.NoError(t, kb.Up("ControlOrMeta"))

				return nil
			},
		)
		require.NoError(t, err)

		// Wait for the new tab to complete loading.
		assert.NoError(t, newTab.WaitForLoadState("load", common.NewFrameWaitForLoadStateOptions(p.MainFrame().Timeout())))

		// Make sure the newTab has a different h1 heading.
		text, err = newTab.Locator("h1", nil).InnerText(nil)
		assert.NoError(t, err)
		assert.Equal(t, "Page 2", text)

		// Make sure there are two pages open.
		pp := bc.Pages()
		assert.Len(t, pp, 2)
	})

	t.Run("meta", func(t *testing.T) {
		t.Parallel()
		t.Skip("FIXME") // See https://go.k6.io/k6/js/modules/k6/browser/issues/424
		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		kb := p.GetKeyboard()

		err := p.SetContent(`<input>`, nil)
		require.NoError(t, err)
		el, err := p.Query("input")
		require.NoError(t, err)
		require.NoError(t, p.Focus("input", common.NewFrameBaseOptions(p.MainFrame().Timeout())))

		require.NoError(t, kb.Press("Shift+KeyA", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Shift+b", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Shift+C", common.KeyboardOptions{}))

		v, err := el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
		require.NoError(t, err)
		require.Equal(t, "AbC", v)

		metaKey := "Control"
		if runtime.GOOS == "darwin" {
			metaKey = "Meta"
		}
		require.NoError(t, kb.Press(metaKey+"+A", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Delete", common.KeyboardOptions{}))
		v, err = el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
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
		require.NoError(t, p.Focus("textarea", common.NewFrameBaseOptions(p.MainFrame().Timeout())))

		require.NoError(t, kb.Type("L+m+KeyN", common.KeyboardOptions{}))
		v, err := el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
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
		require.NoError(t, p.Focus("textarea", common.NewFrameBaseOptions(p.MainFrame().Timeout())))

		require.NoError(t, kb.Press("C", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("d", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("KeyE", common.KeyboardOptions{}))

		require.NoError(t, kb.Down("Shift"))
		require.NoError(t, kb.Down("f"))
		require.NoError(t, kb.Up("f"))
		require.NoError(t, kb.Down("G"))
		require.NoError(t, kb.Up("G"))
		require.NoError(t, kb.Down("KeyH"))
		require.NoError(t, kb.Up("KeyH"))
		require.NoError(t, kb.Up("Shift"))

		v, err := el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
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
		require.NoError(t, p.Focus("textarea", common.NewFrameBaseOptions(p.MainFrame().Timeout())))

		require.NoError(t, kb.Down("Shift"))
		require.NoError(t, kb.Type("oPqR", common.KeyboardOptions{}))
		require.NoError(t, kb.Up("Shift"))

		v, err := el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
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
		require.NoError(t, p.Focus("textarea", common.NewFrameBaseOptions(p.MainFrame().Timeout())))

		require.NoError(t, kb.Type("Hello", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Enter", common.KeyboardOptions{}))
		require.NoError(t, kb.Press("Enter", common.KeyboardOptions{}))
		require.NoError(t, kb.Type("World!", common.KeyboardOptions{}))
		v, err := el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
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
		require.NoError(t, p.Focus("input", common.NewFrameBaseOptions(p.MainFrame().Timeout())))

		require.NoError(t, kb.Type("Hello World!", common.KeyboardOptions{}))
		v, err := el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
		require.NoError(t, err)
		require.Equal(t, "Hello World!", v)

		require.NoError(t, kb.Press("ArrowLeft", common.KeyboardOptions{}))
		// Should hold the key until Up() is called.
		require.NoError(t, kb.Down("Shift"))
		for i := 0; i < len(" World"); i++ {
			require.NoError(t, kb.Press("ArrowLeft", common.KeyboardOptions{}))
		}
		// Should release the key but the selection should remain active.
		require.NoError(t, kb.Up("Shift"))
		// Should delete the selection.
		require.NoError(t, kb.Press("Backspace", common.KeyboardOptions{}))

		require.NoError(t, err)
		require.NoError(t, err)
		assert.Equal(t, "Hello World!", v)
	})
}
