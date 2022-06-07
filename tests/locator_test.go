package tests

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestLocatorClick(t *testing.T) {
	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	// Selecting a single element and clicking on it is OK.
	t.Run("ok", func(t *testing.T) {
		result := func() bool {
			cr := p.Evaluate(tb.toGojaValue(`() => window.result`))
			return cr.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}
		link := p.Locator("#link", nil)
		link.Click(nil)
		require.True(t, result(), "could not click the link")
	})
	// There are two links in the document (locators.html).
	// The strict mode should disallow selecting multiple elements.
	t.Run("strict", func(t *testing.T) {
		link := p.Locator("a", nil)
		require.Panics(t, func() { link.Click(nil) })
	})
}

func TestLocatorDblclick(t *testing.T) {
	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	// Selecting a single element and clicking on it is OK.
	t.Run("ok", func(t *testing.T) {
		dblclick := func() bool {
			cr := p.Evaluate(tb.toGojaValue(`() => window.dblclick`))
			return cr.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}
		link := p.Locator("#link", nil)
		link.Dblclick(nil)
		require.True(t, dblclick(), "could not double click the link")
	})
	// There are two links in the document (locators.html).
	// The strict mode should disallow selecting multiple elements.
	t.Run("strict", func(t *testing.T) {
		link := p.Locator("a", nil)
		require.Panics(t, func() { link.Dblclick(nil) })
	})
}

func TestLocatorCheck(t *testing.T) {
	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("check", func(t *testing.T) {
		check := func() bool {
			cr := p.Evaluate(tb.toGojaValue(`() => window.check`))
			return cr.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}

		cb := p.Locator("#inputCheckbox", nil)
		require.False(t, check(), "should be unchecked first")

		cb.Check(nil)
		require.True(t, check(), "could not check the input box")

		cb.Uncheck(nil)
		require.False(t, check(), "could not uncheck the input box")
	})
	t.Run("is_checked", func(t *testing.T) {
		cb := p.Locator("#inputCheckbox", nil)

		cb.Check(nil)
		require.True(t, cb.IsChecked(nil))

		cb.Uncheck(nil)
		require.False(t, cb.IsChecked(nil))
	})
	// There are multiple input boxes in the document (locators.html).
	// The strict mode should disallow selecting multiple elements.
	t.Run("strict", func(t *testing.T) {
		input := p.Locator("input", nil)
		require.Panics(t, func() { input.Check(nil) }, "should not select multiple elements")
		require.Panics(t, func() { input.Uncheck(nil) }, "should not select multiple elements")
		require.Panics(t, func() { input.IsChecked(nil) }, "should not select multiple elements")
	})
}

func TestLocatorIsEditable(t *testing.T) {
	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("editable", func(t *testing.T) {
		el := p.Locator("#inputText", nil)
		require.True(t, el.IsEditable(nil))

		p.Evaluate(tb.toGojaValue(`() => document.getElementById('inputText').readOnly = true`))
		require.False(t, el.IsEditable(nil))
	})
	t.Run("strict", func(t *testing.T) {
		input := p.Locator("input", nil)
		require.Panics(t, func() { input.IsEditable(nil) }, "should not select multiple elements")
	})
}
