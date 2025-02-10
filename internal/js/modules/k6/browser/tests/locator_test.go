// practically none of this work on windows
//go:build !windows

package tests

import (
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.

// Note:
// We skip adding t.Parallel to subtests because sobek or our code might race.

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
				check := func() bool {
					v, err := p.Evaluate(`() => window.check`)
					require.NoError(t, err)
					return asBool(t, v)
				}
				t.Run("check", func(t *testing.T) {
					l := p.Locator("#inputCheckbox", nil)
					require.False(t, check(), "should be unchecked first")
					require.NoError(t, l.Check(nil))
					require.True(t, check(), "cannot not check the input box")
					require.NoError(t, l.Uncheck(nil))
					require.False(t, check(), "cannot not uncheck the input box")
				})
				t.Run("setChecked", func(t *testing.T) {
					l := p.Locator("#inputCheckbox", nil)
					require.False(t, check(), "should be unchecked first")
					require.NoError(t, l.SetChecked(true, nil))
					require.True(t, check(), "cannot not check the input box")
					require.NoError(t, l.SetChecked(false, nil))
					require.False(t, check(), "cannot not uncheck the input box")
				})
				t.Run("is_checked", func(t *testing.T) {
					l := p.Locator("#inputCheckbox", nil)
					require.NoError(t, l.Check(nil))
					checked, err := l.IsChecked(nil)
					require.NoError(t, err)
					require.True(t, checked)

					require.NoError(t, l.Uncheck(nil))
					checked, err = l.IsChecked(nil)
					require.NoError(t, err)
					require.False(t, checked)
				})
			},
		},
		{
			"Clear", func(_ *testBrowser, p *common.Page) {
				const value = "fill me up"
				l := p.Locator("#inputText", nil)

				require.NoError(t, l.Fill(value, nil))
				inputValue, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, value, inputValue)

				err = l.Clear(common.NewFrameFillOptions(l.Timeout()))
				assert.NoError(t, err)
				inputValue, err = p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				assert.Equal(t, "", inputValue)
			},
		},
		{
			"Click", func(_ *testBrowser, p *common.Page) {
				l := p.Locator("#link", nil)
				err := l.Click(common.NewFrameClickOptions(l.Timeout()))
				require.NoError(t, err)
				v, err := p.Evaluate(`() => window.result`)
				require.NoError(t, err)
				require.True(t, asBool(t, v), "cannot not click the link")
			},
		},
		{
			"Dblclick", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("#linkdbl", nil)
				require.NoError(t, lo.Dblclick(nil))

				v, err := p.Evaluate(`() => window.dblclick`)
				require.NoError(t, err)
				require.True(t, asBool(t, v), "cannot double click the link")
			},
		},
		{
			"DispatchEvent", func(_ *testBrowser, p *common.Page) {
				result := func() bool {
					v, err := p.Evaluate(`() => window.result`)
					require.NoError(t, err)
					return asBool(t, v)
				}
				require.False(t, result(), "should not be clicked first")

				lo := p.Locator("#link", nil)
				opts := common.NewFrameDispatchEventOptions(0) // no timeout
				require.NoError(t, lo.DispatchEvent("click", "mouseevent", opts))
				require.True(t, result(), "cannot not dispatch event")
			},
		},
		{
			"Fill", func(_ *testBrowser, p *common.Page) {
				const value = "fill me up"
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Fill(value, nil))
				inputValue, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, value, inputValue)
			},
		},
		{
			"FillTextarea", func(_ *testBrowser, p *common.Page) {
				const value = "fill me up"
				lo := p.Locator("textarea", nil)
				require.NoError(t, lo.Fill(value, nil))
				inputValue, err := p.InputValue("textarea", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, value, inputValue)
			},
		},
		{
			"FillParagraph", func(_ *testBrowser, p *common.Page) {
				const value = "fill me up"
				lo := p.Locator("#firstParagraph", nil)
				require.NoError(t, lo.Fill(value, nil))
				textContent, ok, err := p.TextContent("#firstParagraph", common.NewFrameTextContentOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, value, textContent)
				lo = p.Locator("#secondParagraph", nil)
				require.Error(t, lo.Fill(value, nil))
			},
		},
		{
			"Focus", func(_ *testBrowser, p *common.Page) {
				focused := func() bool {
					v, err := p.Evaluate(
						`() => document.activeElement == document.getElementById('inputText')`,
					)
					require.NoError(t, err)
					return asBool(t, v)
				}
				lo := p.Locator("#inputText", nil)
				require.False(t, focused(), "should not be focused first")
				require.NoError(t, lo.Focus(nil))
				require.True(t, focused(), "should be focused")
			},
		},
		{
			"GetAttribute", func(_ *testBrowser, p *common.Page) {
				l := p.Locator("#inputText", nil)
				v, ok, err := l.GetAttribute("value", nil)
				require.NoError(t, err)
				require.NotNil(t, v)
				require.True(t, ok)
				require.Equal(t, "something", v)
			},
		},
		{
			"Hover", func(_ *testBrowser, p *common.Page) {
				result := func() bool {
					v, err := p.Evaluate(`() => window.result`)
					require.NoError(t, err)
					return asBool(t, v)
				}
				require.False(t, result(), "should not be hovered first")
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Hover(nil))
				require.True(t, result(), "should be hovered")
			},
		},
		{
			"InnerHTML", func(_ *testBrowser, p *common.Page) {
				html, err := p.Locator("#divHello", nil).InnerHTML(nil)
				require.NoError(t, err)
				require.Equal(t, `<span>hello</span>`, html)
			},
		},
		{
			"InnerText", func(_ *testBrowser, p *common.Page) {
				text, err := p.Locator("#divHello > span", nil).InnerText(nil)
				require.NoError(t, err)
				require.Equal(t, `hello`, text)
			},
		},
		{
			"InputValue", func(_ *testBrowser, p *common.Page) {
				t.Run("input", func(t *testing.T) {
					v, err := p.Locator("#inputText", nil).InputValue(nil)
					require.NoError(t, err)
					require.Equal(t, "something", v)
				})
				t.Run("textarea", func(t *testing.T) {
					v, err := p.Locator("textarea", nil).InputValue(nil)
					require.NoError(t, err)
					require.Equal(t, "text area", v)
				})
				t.Run("select", func(t *testing.T) {
					v, err := p.Locator("#selectElement", nil).InputValue(nil)
					require.NoError(t, err)
					require.Equal(t, "option text", v)
				})
			},
		},
		{
			"Press", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Press("x", nil))
				inputValue, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, "xsomething", inputValue)
			},
		},
		{
			"SelectOption", func(tb *testBrowser, p *common.Page) {
				l := p.Locator("#selectElement", nil)
				rv, err := l.SelectOption(tb.toSobekValue(`option text 2`), nil)
				require.NoError(t, err)
				require.Len(t, rv, 1)
				require.Equal(t, "option text 2", rv[0])
			},
		},
		{
			"Tap", func(_ *testBrowser, p *common.Page) {
				result := func() bool {
					v, err := p.Evaluate(`() => window.result`)
					require.NoError(t, err)
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
			"TextContent", func(_ *testBrowser, p *common.Page) {
				text, ok, err := p.Locator("#divHello", nil).TextContent(nil)
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, `hello`, text)
			},
		},
		{
			"Type", func(_ *testBrowser, p *common.Page) {
				lo := p.Locator("#inputText", nil)
				require.NoError(t, lo.Type("real ", nil))
				inputValue, err := p.InputValue("#inputText", common.NewFrameInputValueOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
				require.Equal(t, "real something", inputValue)
			},
		},
		{
			"WaitFor state:visible", func(tb *testBrowser, p *common.Page) {
				opts := tb.toSobekValue(jsFrameBaseOpts{Timeout: "100"})
				lo := p.Locator("#link", nil)
				require.NoError(t, lo.WaitFor(opts))
			},
		},
		{
			"WaitFor state:attached", func(tb *testBrowser, p *common.Page) {
				opts := tb.toSobekValue(jsFrameWaitForSelectorOpts{
					jsFrameBaseOpts: jsFrameBaseOpts{Timeout: "100"},
					State:           "attached",
				})
				lo := p.Locator("#link", nil)
				require.NoError(t, lo.WaitFor(opts))
			},
		},
		{
			"WaitFor state:hidden", func(tb *testBrowser, p *common.Page) {
				opts := tb.toSobekValue(jsFrameWaitForSelectorOpts{
					jsFrameBaseOpts: jsFrameBaseOpts{Timeout: "100"},
					State:           "hidden",
				})
				lo := p.Locator("#inputHiddenText", nil)
				require.NoError(t, lo.WaitFor(opts))
			},
		},
		{
			"WaitFor state:detached", func(tb *testBrowser, p *common.Page) {
				opts := tb.toSobekValue(jsFrameWaitForSelectorOpts{
					jsFrameBaseOpts: jsFrameBaseOpts{Timeout: "100"},
					State:           "detached",
				})
				lo := p.Locator("#nonExistingElement", nil)
				require.NoError(t, lo.WaitFor(opts))
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

	timeout := func(tb *testBrowser) sobek.Value {
		return tb.toSobekValue(jsFrameBaseOpts{Timeout: "100"})
	}
	sanityTests := []struct {
		name string
		do   func(*common.Locator, *testBrowser) error
	}{
		{
			"Check", func(l *common.Locator, tb *testBrowser) error {
				return l.Check(timeout(tb))
			},
		},
		{
			"Clear", func(l *common.Locator, _ *testBrowser) error {
				opts := common.NewFrameFillOptions(100 * time.Millisecond)
				return l.Clear(opts)
			},
		},
		{
			"Click", func(l *common.Locator, _ *testBrowser) error {
				opts := common.NewFrameClickOptions(100 * time.Millisecond)
				return l.Click(opts)
			},
		},
		{
			"Dblclick", func(l *common.Locator, tb *testBrowser) error {
				return l.Dblclick(timeout(tb))
			},
		},
		{
			"DispatchEvent", func(l *common.Locator, _ *testBrowser) error {
				opts := common.NewFrameDispatchEventOptions(100 * time.Millisecond)
				return l.DispatchEvent("click", "mouseevent", opts)
			},
		},
		{
			"Focus", func(l *common.Locator, tb *testBrowser) error {
				return l.Focus(timeout(tb))
			},
		},
		{
			"Fill", func(l *common.Locator, tb *testBrowser) error {
				return l.Fill("fill me up", timeout(tb))
			},
		},
		{
			"GetAttribute", func(l *common.Locator, tb *testBrowser) error {
				_, _, err := l.GetAttribute("value", timeout(tb))
				return err
			},
		},
		{
			"Hover", func(l *common.Locator, tb *testBrowser) error {
				return l.Hover(timeout(tb))
			},
		},
		{
			"InnerHTML", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.InnerHTML(timeout(tb))
				return err
			},
		},
		{
			"InnerText", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.InnerText(timeout(tb))
				return err
			},
		},
		{
			"InputValue", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.InputValue(timeout(tb))
				return err
			},
		},
		{
			"Press", func(l *common.Locator, tb *testBrowser) error {
				return l.Press("a", timeout(tb))
			},
		},
		{
			"SetChecked", func(l *common.Locator, tb *testBrowser) error {
				return l.SetChecked(true, timeout(tb))
			},
		},
		{
			"SelectOption", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.SelectOption(tb.toSobekValue(""), timeout(tb))
				return err
			},
		},
		{
			"Tap", func(l *common.Locator, _ *testBrowser) error {
				opts := common.NewFrameTapOptions(100 * time.Millisecond)
				return l.Tap(opts)
			},
		},
		{
			"Type", func(l *common.Locator, tb *testBrowser) error {
				return l.Type("a", timeout(tb))
			},
		},
		{
			"TextContent", func(l *common.Locator, tb *testBrowser) error {
				_, _, err := l.TextContent(timeout(tb))
				return err
			},
		},
		{
			"Uncheck", func(l *common.Locator, tb *testBrowser) error {
				return l.Uncheck(timeout(tb))
			},
		},
		{
			"WaitFor", func(l *common.Locator, tb *testBrowser) error {
				return l.WaitFor(timeout(tb))
			},
		},
	}
	for _, tt := range sanityTests {
		tt := tt
		t.Run("timeout/"+tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			p := tb.NewPage(nil)
			err := p.SetContent("<html></html>", nil)
			require.NoError(t, err)
			require.Error(t, tt.do(p.Locator("NOTEXIST", nil), tb))
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

			require.Error(t, tt.do(p.Locator("a", nil), tb))
		})
	}
}

func TestLocatorElementState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state, eval string
		query       func(*common.Locator) (bool, error)
	}{
		{
			"disabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsDisabled(nil)
				return !resp, err
			},
		},
		{
			"enabled",
			`() => document.getElementById('inputText').disabled = true`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsEnabled(nil)
				return resp, err
			},
		},
		{
			"hidden",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsHidden()
				return !resp, err
			},
		},
		{
			"readOnly",
			`() => document.getElementById('inputText').readOnly = true`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsEditable(nil)
				return resp, err
			},
		},
		{
			"visible",
			`() => document.getElementById('inputText').style.visibility = 'hidden'`,
			func(l *common.Locator) (bool, error) {
				resp, err := l.IsVisible()
				return resp, err
			},
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
			result, err := tt.query(l)
			require.NoError(t, err)
			require.True(t, result)

			_, err = p.Evaluate(tt.eval)
			require.NoError(t, err)
			result, err = tt.query(l)
			require.NoError(t, err)
			require.False(t, result)
		})
	}

	timeout := func(tb *testBrowser) sobek.Value {
		return tb.toSobekValue(jsFrameBaseOpts{Timeout: "100"})
	}
	sanityTests := []struct {
		name string
		do   func(*common.Locator, *testBrowser) error
	}{
		{
			"IsChecked", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.IsChecked(timeout(tb))
				return err
			},
		},
		{
			"IsEditable", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.IsEditable(timeout(tb))
				return err
			},
		},
		{
			"IsEnabled", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.IsEnabled(timeout(tb))
				return err
			},
		},
		{
			"IsDisabled", func(l *common.Locator, tb *testBrowser) error {
				_, err := l.IsDisabled(timeout(tb))
				return err
			},
		},
	}
	for _, tt := range sanityTests {
		tt := tt
		t.Run("timeout/"+tt.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t)
			p := tb.NewPage(nil)
			err := p.SetContent("<html></html>", nil)
			require.NoError(t, err)
			require.Error(t, tt.do(p.Locator("NOTEXIST", nil), tb))
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
			require.Error(t, tt.do(p.Locator("a", nil), tb))
		})
	}
}

func TestLocatorPress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)

	p := tb.NewPage(nil)

	err := p.SetContent(`<input id="text1">`, nil)
	require.NoError(t, err)

	l := p.Locator("#text1", nil)

	require.NoError(t, l.Press("Shift+KeyA", nil))
	require.NoError(t, l.Press("KeyB", nil))
	require.NoError(t, l.Press("Shift+KeyC", nil))

	v, err := l.InputValue(nil)
	require.NoError(t, err)
	require.Equal(t, "AbC", v)
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

func TestSelectOption(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t,
		withFileServer(),
	)
	defer tb.Browser.Close()

	vu, _, _, cleanUp := startIteration(t)
	defer cleanUp()

	got := vu.RunPromise(t, `
		const page = await browser.newPage();

		await page.goto('%s');

		const options = page.locator('#numbers-options');

		await options.selectOption({label:'Five'});
		let selectedValue = await options.inputValue();
		if (selectedValue !== 'five') {
			throw new Error('Expected "five" but got ' + selectedValue);
		}

		await options.selectOption({index:5});
		selectedValue = await options.inputValue();
		if (selectedValue !== 'five') {
			throw new Error('Expected "five" but got ' + selectedValue);
		}

		await options.selectOption({value:'four'});
		selectedValue = await options.inputValue();
		if (selectedValue !== 'four') {
			throw new Error('Expected "four" but got ' + selectedValue);
		}

		await options.selectOption([{label:'One'}]);
		selectedValue = await options.inputValue();
		if (selectedValue !== 'one') {
			throw new Error('Expected "one" but got ' + selectedValue);
		}

		await options.selectOption(['two']); // Value
		selectedValue = await options.inputValue();
		if (selectedValue !== 'two') {
			throw new Error('Expected "two" but got ' + selectedValue);
		}

		await options.selectOption('five'); // Value
		selectedValue = await options.inputValue();
		if (selectedValue !== 'five') {
			throw new Error('Expected "five" but got ' + selectedValue);
		}
	`, tb.staticURL("select_options.html"))
	assert.Equal(t, sobek.Undefined(), got.Result())
}
