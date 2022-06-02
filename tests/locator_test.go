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

	// Selecting a single element and checking it is OK.
	t.Run("ok", func(t *testing.T) {
		check := func() bool {
			cr := p.Evaluate(tb.toGojaValue(`() => window.check`))
			return cr.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}
		input := p.Locator("#input", nil)
		input.Check(nil)
		require.True(t, check(), "could not check the input box")
	})
	// There are two input boxes in the document (locators.html).
	// The strict mode should disallow selecting multiple elements.
	t.Run("strict", func(t *testing.T) {
		input := p.Locator("input", nil)
		require.Panics(t,
			func() { input.Check(nil) },
			"should not select multiple elements",
		)
	})
}

func TestLocatorUncheck(t *testing.T) {
	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	// Selecting a single element and checking it is OK.
	t.Run("ok", func(t *testing.T) {
		uncheck := func() bool {
			cr := p.Evaluate(tb.toGojaValue(`() => window.uncheck`))
			return cr.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}
		input := p.Locator("#checkedInput", nil)
		input.Uncheck(nil)
		require.True(t, uncheck(), "could not uncheck the input box")
	})
	// There are two checked input boxes in the document (locators.html).
	// The strict mode should disallow selecting multiple elements.
	t.Run("strict", func(t *testing.T) {
		input := p.Locator("input", nil)
		require.Panics(t,
			func() { input.Uncheck(nil) },
			"should not select multiple elements",
		)
	})
}
