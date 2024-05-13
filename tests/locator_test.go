package tests

import (
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
)

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.

// Note:
// We skip adding t.Parallel to subtests because goja or our code might race.

type jsFrameWaitForSelectorOpts struct {
	jsFrameBaseOpts

	State string
}

func TestLocator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		do   func(*testBrowser, *common.Page)
	}{
		{
			"Check", func(_ *testBrowser, p *common.Page) {
				t.Run("check", func(t *testing.T) {
					check := func() bool {
						v := p.Evaluate(`() => window.check`)
						return asBool(t, v)
					}
					l := p.Locator("#inputCheckbox", nil)
					require.False(t, check(), "should be unchecked first")
					require.NoError(t, l.Check(nil))
					require.True(t, check(), "cannot not check the input box")
					require.NoError(t, l.Uncheck(nil))
					require.False(t, check(), "cannot not uncheck the input box")
				})
				t.Run("is_checked", func(t *testing.T) {
					l := p.Locator("#inputCheckbox", nil)
					require.NoError(t, l.Check(nil))
					require.True(t, l.IsChecked(nil))
					require.NoError(t, l.Uncheck(nil))
					require.False(t, l.IsChecked(nil))
				})
			},
		},
		{
			"Clear", func(tb *testBrowser, p *common.Page) {
				const value = "fill me up"
				l := p.Locator("#inputText", nil)

				l.Fill(value, nil)
				require.Equal(t, value, p.InputValue("#inputText", nil))

				err := l.Clear(common.NewFrameFillOptions(l.Timeout()))
				assert.NoError(t, err)
				assert.Equal(t, "", p.InputValue("#inputText", nil))
			},
		},
		{
			"Click", func(tb *testBrowser, p *common.Page) {
				l := p.Locator("#link", nil)
				err := l.Click(common.NewFrameClickOptions(l.Timeout()))
				require.NoError(t, err)
				v := p.Evaluate(`() => window.result`)
				require.True(t, asBool(t, v), "cannot not click the link")
			},
		},
		{
			"Dblclick", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("#linkdbl", nil)
				require.NoError(t, lo.Dblclick(nil))

				v := p.Evaluate(`() => window.dblclick`)
				require.True(t, asBool(t, v), "cannot double click the link")
			},
		},
		{
			"DispatchEvent", func(tb *testBrowser, p *common.Page) {
				result := func() bool {
					v := p.Evaluate(`() => window.result`)
					return asBool(t, v)
				}
				require.False(t, result(), "should not be clicked first")
				opts := common.NewFrameDispatchEventOptions(0) // no timeout
				err := p.Locator("#link", nil).DispatchEvent("click", "mouseevent", opts)
				require.NoError(t, err)
				require.True(t, result(), "cannot not dispatch event")
			},
		},
		{
			"Fill", func(tb *testBrowser, p *common.Page) {
				const value = "fill me up"
				p.Locator("#inputText", nil).Fill(value, nil)
				require.Equal(t, value, p.InputValue("#inputText", nil))
			},
		},
		{
			"FillTextarea", func(tb *testBrowser, p *common.Page) {
				const value = "fill me up"
				p.Locator("textarea", nil).Fill(value, nil)
				require.Equal(t, value, p.InputValue("textarea", nil))
			},
		},
		{
			"FillParagraph", func(tb *testBrowser, p *common.Page) {
				const value = "fill me up"
				p.Locator("#firstParagraph", nil).Fill(value, nil)
				require.Equal(t, value, p.TextContent("#firstParagraph", nil))
				l := p.Locator("#secondParagraph", nil)
				assert.Panics(t, func() { l.Fill(value, nil) }, "should panic")
			},
		},
		{
			"Focus", func(tb *testBrowser, p *common.Page) {
				focused := func() bool {
					v := p.Evaluate(
						`() => document.activeElement == document.getElementById('inputText')`,
					)
					return asBool(t, v)
				}
				l := p.Locator("#inputText", nil)
				require.False(t, focused(), "should not be focused first")
				l.Focus(nil)
				require.True(t, focused(), "should be focused")
			},
		},
		{
			"GetAttribute", func(tb *testBrowser, p *common.Page) {
				l := p.Locator("#inputText", nil)
				v := l.GetAttribute("value", nil)
				require.NotNil(t, v)
				require.Equal(t, "something", v)
			},
		},
		{
			"Hover", func(tb *testBrowser, p *common.Page) {
				result := func() bool {
					v := p.Evaluate(`() => window.result`)
					return asBool(t, v)
				}
				require.False(t, result(), "should not be hovered first")
				p.Locator("#inputText", nil).Hover(nil)
				require.True(t, result(), "should be hovered")
			},
		},
		{
			"InnerHTML", func(tb *testBrowser, p *common.Page) {
				require.Equal(t, `<span>hello</span>`, p.Locator("#divHello", nil).InnerHTML(nil))
			},
		},
		{
			"InnerText", func(tb *testBrowser, p *common.Page) {
				require.Equal(t, `hello`, p.Locator("#divHello > span", nil).InnerText(nil))
			},
		},
		{
			"InputValue", func(tb *testBrowser, p *common.Page) {
				t.Run("input", func(t *testing.T) {
					require.Equal(t, "something", p.Locator("#inputText", nil).InputValue(nil))
				})
				t.Run("textarea", func(t *testing.T) {
					require.Equal(t, "text area", p.Locator("textarea", nil).InputValue(nil))
				})
				t.Run("select", func(t *testing.T) {
					require.Equal(t, "option text", p.Locator("#selectElement", nil).InputValue(nil))
				})
			},
		},
		{
			"Press", func(tb *testBrowser, p *common.Page) {
				p.Locator("#inputText", nil).Press("x", nil)
				require.Equal(t, "xsomething", p.InputValue("#inputText", nil))
			},
		},
		{
			"SelectOption", func(tb *testBrowser, p *common.Page) {
				l := p.Locator("#selectElement", nil)
				rv := l.SelectOption(tb.toGojaValue(`option text 2`), nil)
				require.Len(t, rv, 1)
				require.Equal(t, "option text 2", rv[0])
			},
		},
		{
			"Tap", func(_ *testBrowser, p *common.Page) {
				result := func() bool {
					v := p.Evaluate(`() => window.result`)
					return asBool(t, v)
				}
				require.False(t, result(), "should not be tapped first")
				opts := common.NewFrameTapOptions(common.DefaultTimeout)
				err := p.Locator("#inputText", nil).Tap(opts)
				require.NoError(t, err)
				require.True(t, result(), "should be tapped")
			},
		},
		{
			"TextContent", func(tb *testBrowser, p *common.Page) {
				require.Equal(t, `hello`, p.Locator("#divHello", nil).TextContent(nil))
			},
		},
		{
			"Type", func(tb *testBrowser, p *common.Page) {
				p.Locator("#inputText", nil).Type("real ", nil)
				require.Equal(t, "real something", p.InputValue("#inputText", nil))
			},
		},
		{
			"WaitFor state:visible", func(tb *testBrowser, p *common.Page) {
				opts := tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})
				require.NotPanics(t, func() { p.Locator("#link", nil).WaitFor(opts) })
			},
		},
		{
			"WaitFor state:attached", func(tb *testBrowser, p *common.Page) {
				opts := tb.toGojaValue(jsFrameWaitForSelectorOpts{
					jsFrameBaseOpts: jsFrameBaseOpts{Timeout: "100"},
					State:           "attached",
				})
				require.NotPanics(t, func() { p.Locator("#link", nil).WaitFor(opts) })
			},
		},
		{
			"WaitFor state:hidden", func(tb *testBrowser, p *common.Page) {
				opts := tb.toGojaValue(jsFrameWaitForSelectorOpts{
					jsFrameBaseOpts: jsFrameBaseOpts{Timeout: "100"},
					State:           "hidden",
				})
				require.NotPanics(t, func() { p.Locator("#inputHiddenText", nil).WaitFor(opts) })
			},
		},
		{
			"WaitFor state:detached", func(tb *testBrowser, p *common.Page) {
				opts := tb.toGojaValue(jsFrameWaitForSelectorOpts{
					jsFrameBaseOpts: jsFrameBaseOpts{Timeout: "100"},
					State:           "detached",
				})
				require.NotPanics(t, func() { p.Locator("#nonExistingElement", nil).WaitFor(opts) })
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)
			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("locators.html"),
				opts,
			)
			tt.do(tb, p)
			require.NoError(t, err)
		})
	}

	timeout := func(tb *testBrowser) goja.Value {
		return tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})
	}
	sanityTests := []struct {
		name string
		do   func(*common.Locator, *testBrowser)
	}{
		{
			"Check", func(l *common.Locator, tb *testBrowser) {
				// TODO: remove panic and update tests when all locator methods return error.
				if err := l.Check(timeout(tb)); err != nil {
					panic(err)
				}
			},
		},
		{
			"Clear", func(l *common.Locator, tb *testBrowser) {
				err := l.Clear(common.NewFrameFillOptions(100 * time.Millisecond))
				if err != nil {
					// TODO: remove panic and update tests when all locator methods return error.
					panic(err)
				}
			},
		},
		{
			"Click", func(l *common.Locator, tb *testBrowser) {
				err := l.Click(common.NewFrameClickOptions(100 * time.Millisecond))
				if err != nil {
					// TODO: remove panic and update tests when all locator methods return error.
					panic(err)
				}
			},
		},
		{
			"Dblclick", func(l *common.Locator, tb *testBrowser) {
				if err := l.Dblclick(timeout(tb)); err != nil {
					// TODO: remove panic and update tests when all locator methods return error.
					panic(err)
				}
			},
		},
		{
			"DispatchEvent", func(l *common.Locator, tb *testBrowser) {
				err := l.DispatchEvent("click", "mouseevent", common.NewFrameDispatchEventOptions(100*time.Millisecond))
				if err != nil {
					// TODO: remove panic and update tests when all locator methods return error.
					panic(err)
				}
			},
		},
		{
			"Focus", func(l *common.Locator, tb *testBrowser) { l.Focus(timeout(tb)) },
		},
		{
			"Fill", func(l *common.Locator, tb *testBrowser) { l.Fill("fill me up", timeout(tb)) },
		},
		{
			"GetAttribute", func(l *common.Locator, tb *testBrowser) { l.GetAttribute("value", timeout(tb)) },
		},
		{
			"Hover", func(l *common.Locator, tb *testBrowser) { l.Hover(timeout(tb)) },
		},
		{
			"InnerHTML", func(l *common.Locator, tb *testBrowser) { l.InnerHTML(timeout(tb)) },
		},
		{
			"InnerText", func(l *common.Locator, tb *testBrowser) { l.InnerText(timeout(tb)) },
		},
		{
			"InputValue", func(l *common.Locator, tb *testBrowser) { l.InputValue(timeout(tb)) },
		},
		{
			"Press", func(l *common.Locator, tb *testBrowser) { l.Press("a", timeout(tb)) },
		},
		{
			"SelectOption", func(l *common.Locator, tb *testBrowser) { l.SelectOption(tb.toGojaValue(""), timeout(tb)) },
		},
		{
			"Tap", func(l *common.Locator, _ *testBrowser) {
				if err := l.Tap(common.NewFrameTapOptions(100 * time.Millisecond)); err != nil {
					// TODO: remove panic and update tests when all locator methods return error.
					panic(err)
				}
			},
		},
		{
			"Type", func(l *common.Locator, tb *testBrowser) {
				err := l.Type("a", timeout(tb))
				if err != nil {
					// TODO: remove panic and update tests when all locator methods return error.
					panic(err)
				}
			},
		},
		{
			"TextContent", func(l *common.Locator, tb *testBrowser) { l.TextContent(timeout(tb)) },
		},
		{
			"Uncheck", func(l *common.Locator, tb *testBrowser) {
				if err := l.Uncheck(timeout(tb)); err != nil {
					// TODO: remove panic and update tests when all locator methods return error.
					panic(err)
				}
			},
		},
		{
			"WaitFor", func(l *common.Locator, tb *testBrowser) { l.WaitFor(timeout(tb)) },
		},
	}
	for _, tt := range sanityTests {
		tt := tt
		t.Run("timeout/"+tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			p := tb.NewPage(nil)
			p.SetContent("<html></html>", nil)
			assert.Panics(t, func() { tt.do(p.Locator("NOTEXIST", nil), tb) })
		})
	}

	for _, tt := range sanityTests {
		tt := tt
		t.Run("strict/"+tt.name, func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("locators.html"),
				opts,
			)
			require.NoError(t, err)

			assert.Panics(t, func() {
				tt.do(p.Locator("a", nil), tb)
			})
		})
	}
}

func TestLocatorElementState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state, eval string
		query       func(*common.Locator) bool
	}{
		{
			"disabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l *common.Locator) bool { return !l.IsDisabled(nil) },
		},
		{
			"enabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l *common.Locator) bool { return l.IsEnabled(nil) },
		},
		{
			"hidden",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l *common.Locator) bool { resp, _ := l.IsHidden(); return !resp },
		},
		{
			"readOnly",
			`() => document.getElementById('inputText').readOnly = true`,
			func(l *common.Locator) bool { return l.IsEditable(nil) },
		},
		{
			"visible",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l *common.Locator) bool { resp, _ := l.IsVisible(); return resp },
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.state, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("locators.html"),
				opts,
			)
			require.NoError(t, err)

			l := p.Locator("#inputText", nil)
			require.True(t, tt.query(l))

			p.Evaluate(tt.eval)
			require.False(t, tt.query(l))
			require.NoError(t, err)
		})
	}

	timeout := func(tb *testBrowser) goja.Value {
		return tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})
	}
	sanityTests := []struct {
		name string
		do   func(*common.Locator, *testBrowser)
	}{
		{
			"IsChecked", func(l *common.Locator, tb *testBrowser) { l.IsChecked(timeout(tb)) },
		},
		{
			"IsEditable", func(l *common.Locator, tb *testBrowser) { l.IsEditable(timeout(tb)) },
		},
		{
			"IsEnabled", func(l *common.Locator, tb *testBrowser) { l.IsEnabled(timeout(tb)) },
		},
		{
			"IsDisabled", func(l *common.Locator, tb *testBrowser) { l.IsDisabled(timeout(tb)) },
		},
	}
	for _, tt := range sanityTests {
		tt := tt
		t.Run("timeout/"+tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			p := tb.NewPage(nil)
			p.SetContent("<html></html>", nil)
			assert.Panics(t, func() { tt.do(p.Locator("NOTEXIST", nil), tb) })
		})
	}

	for _, tt := range sanityTests {
		tt := tt
		t.Run("strict/"+tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())
			p := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := p.Goto(
				tb.staticURL("locators.html"),
				opts,
			)
			assert.Panics(t, func() {
				tt.do(p.Locator("a", nil), tb)
			})
			require.NoError(t, err)
		})
	}
}

func TestLocatorPress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)

	p := tb.NewPage(nil)

	p.SetContent(`<input id="text1">`, nil)

	l := p.Locator("#text1", nil)

	l.Press("Shift+KeyA", nil)
	l.Press("KeyB", nil)
	l.Press("Shift+KeyC", nil)

	require.Equal(t, "AbC", l.InputValue(nil))
}

func TestLocatorShadowDOM(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewPage(nil)

	popts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := p.Goto(
		tb.staticURL("shadow_dom_link.html"),
		popts,
	)
	require.NoError(t, err)
	err = p.Click("#inner-link", common.NewFrameClickOptions(time.Second))
	require.NoError(t, err)
}
