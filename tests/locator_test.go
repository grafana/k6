package tests

import (
	"testing"

	"github.com/grafana/xk6-browser/api"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.

// Note:
// We skip adding t.Parallel to subtests because goja or our code might race.

func TestLocatorClick(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	// Selecting a single element and clicking on it is OK.
	t.Run("ok", func(t *testing.T) {
		result := func() bool {
			ok := p.Evaluate(tb.toGojaValue(`() => window.result`))
			return ok.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}
		p.Locator("#link", nil).Click(nil)
		require.True(t, result(), "could not click the link")
	})
	// The strict mode should disallow selecting multiple elements.
	t.Run("strict", func(t *testing.T) {
		require.Panics(t, func() { p.Locator("a", nil).Click(nil) })
	})
}

func TestLocatorDblclick(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("ok", func(t *testing.T) {
		dblclick := func() bool {
			ok := p.Evaluate(tb.toGojaValue(`() => window.dblclick`))
			return ok.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}
		p.Locator("#link", nil).Dblclick(nil)
		require.True(t, dblclick(), "could not double click the link")
	})
	t.Run("strict", func(t *testing.T) {
		require.Panics(t, func() { p.Locator("a", nil).Dblclick(nil) })
	})
}

func TestLocatorCheck(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("check", func(t *testing.T) {
		check := func() bool {
			ok := p.Evaluate(tb.toGojaValue(`() => window.check`))
			return ok.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}

		l := p.Locator("#inputCheckbox", nil)
		require.False(t, check(), "should be unchecked first")

		l.Check(nil)
		require.True(t, check(), "could not check the input box")

		l.Uncheck(nil)
		require.False(t, check(), "could not uncheck the input box")
	})
	t.Run("is_checked", func(t *testing.T) {
		l := p.Locator("#inputCheckbox", nil)

		l.Check(nil)
		require.True(t, l.IsChecked(nil))

		l.Uncheck(nil)
		require.False(t, l.IsChecked(nil))
	})
	t.Run("strict", func(t *testing.T) {
		l := p.Locator("input", nil)
		require.Panics(t, func() { l.Check(nil) }, "should not select multiple elements")
		require.Panics(t, func() { l.Uncheck(nil) }, "should not select multiple elements")
		require.Panics(t, func() { l.IsChecked(nil) }, "should not select multiple elements")
	})
}

func TestLocatorElementState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state, eval string
		query       func(api.Locator) bool
	}{
		{
			"readOnly",
			`() => document.getElementById('inputText').readOnly = true`,
			func(l api.Locator) bool { return l.IsEditable(nil) },
		},
		{
			"enabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l api.Locator) bool { return l.IsEnabled(nil) },
		},
		{
			"disabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l api.Locator) bool { return !l.IsDisabled(nil) },
		},
		{
			"visible",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l api.Locator) bool { return l.IsVisible(nil) },
		},
		{
			"hidden",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l api.Locator) bool { return !l.IsHidden(nil) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)
			require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

			l := p.Locator("#inputText", nil)
			require.True(t, tt.query(l))

			p.Evaluate(tb.toGojaValue(tt.eval))
			require.False(t, tt.query(l))
		})
	}

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	for _, tt := range tests {
		t.Run("strict/"+tt.state, func(t *testing.T) {
			require.Panics(t, func() {
				tt.query(p.Locator("input", nil))
			}, "should not select multiple elements")
		})
	}
}

func TestLocatorFill(t *testing.T) {
	t.Parallel()

	const value = "fill me up"

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("ok", func(t *testing.T) {
		p.Locator("#inputText", nil).Fill(value, nil)
		require.Equal(t, value, p.InputValue("#inputText", nil))
	})
	t.Run("strict", func(t *testing.T) {
		require.Panics(t, func() {
			p.Locator("input", nil).Fill(value, nil)
		}, "should not select multiple elements")
	})
}

func TestLocatorFocus(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("ok", func(t *testing.T) {
		focused := func() bool {
			ok := p.Evaluate(tb.toGojaValue(
				`() => document.activeElement == document.getElementById('inputText')`,
			))
			return ok.(goja.Value).ToBoolean() //nolint:forcetypeassert
		}

		l := p.Locator("#inputText", nil)
		require.False(t, focused(), "should not be focused first")

		l.Focus(nil)
		require.True(t, focused(), "should be focused")
	})
	t.Run("strict", func(t *testing.T) {
		require.Panics(t, func() {
			p.Locator("input", nil).Focus(nil)
		}, "should not select multiple elements")
	})
}

//nolint:tparallel
func TestLocatorGetAttribute(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("ok", func(t *testing.T) {
		l := p.Locator("#inputText", nil)
		v := l.GetAttribute("value", nil)
		require.NotNil(t, v)
		require.Equal(t, "something", v.ToString().String())
	})
	t.Run("strict", func(t *testing.T) {
		require.Panics(t, func() {
			p.Locator("input", nil).GetAttribute("value", nil)
		}, "should not select multiple elements")
	})
}

//nolint:tparallel
func TestLocatorInnerHTML(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("ok", func(t *testing.T) {
		require.Equal(t, `<span>hello</span>`, p.Locator("#divHello", nil).InnerHTML(nil))
	})
	t.Run("strict", func(t *testing.T) {
		require.Panics(t, func() {
			p.Locator("div", nil).InnerHTML(nil)
		}, "should not select multiple elements")
	})
}

//nolint:tparallel
func TestLocatorInnerText(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)
	require.NotNil(t, p.Goto(tb.staticURL("/locators.html"), nil))

	t.Run("ok", func(t *testing.T) {
		require.Equal(t, `hello`, p.Locator("#divHello > span", nil).InnerText(nil))
	})
	t.Run("strict", func(t *testing.T) {
		require.Panics(t, func() {
			p.Locator("div", nil).InnerText(nil)
		}, "should not select multiple elements")
	})
}
