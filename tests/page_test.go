package tests

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/writer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
)

type emulateMediaOpts struct {
	Media         string `js:"media"`
	ColorScheme   string `js:"colorScheme"`
	ReducedMotion string `js:"reducedMotion"`
}

type jsFrameBaseOpts struct {
	Timeout string
	Strict  bool
}

const sampleHTML = `<div><b>Test</b><ol><li><i>One</i></li></ol></div>`

func TestNestedFrames(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBufferString("")
	tb := newTestBrowser(t,
		withFileServer(),
		func(tb *testBrowser) {
			if !tb.isBrowserTypeInitialized {
				return
			}
			tb.vu.StateField.Logger.(*logrus.Logger).AddHook(&writer.Hook{Writer: buf, LogLevels: []logrus.Level{logrus.InfoLevel}})
		},
	)
	defer tb.Browser.Close()

	page := tb.NewPage(nil)
	_, err := page.Goto(tb.staticURL("iframe_test_main.html"), nil)
	require.NoError(t, err)

	frame1Handle, err := page.WaitForSelector("iframe[id='iframe1']", nil)
	assert.Nil(t, err)
	assert.NotNil(t, frame1Handle)

	frame1, err := frame1Handle.ContentFrame()
	assert.Nil(t, err)
	assert.NotNil(t, frame1)

	frame2Handle, err := frame1.WaitForSelector("iframe[id='iframe2']", nil)
	assert.Nil(t, err)
	assert.NotNil(t, frame2Handle)

	frame2, err := frame2Handle.ContentFrame()
	assert.Nil(t, err)
	assert.NotNil(t, frame2)

	button1Handle, err := frame2.WaitForSelector("button[id='button1']", nil)
	assert.Nil(t, err)
	assert.NotNil(t, button1Handle)

	err = button1Handle.Click(nil)
	assert.Nil(t, err)

	assert.Contains(t, buf.String(), "button1 clicked")
}

func TestPageEmulateMedia(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.EmulateMedia(tb.toGojaValue(emulateMediaOpts{
		Media:         "print",
		ColorScheme:   "dark",
		ReducedMotion: "reduce",
	}))

	result := p.Evaluate(tb.toGojaValue("() => matchMedia('print').matches"))
	res := tb.asGojaValue(result)
	assert.True(t, res.ToBoolean(), "expected media 'print'")

	result = p.Evaluate(tb.toGojaValue("() => matchMedia('(prefers-color-scheme: dark)').matches"))
	res = tb.asGojaValue(result)
	assert.True(t, res.ToBoolean(), "expected color scheme 'dark'")

	result = p.Evaluate(tb.toGojaValue("() => matchMedia('(prefers-reduced-motion: reduce)').matches"))
	res = tb.asGojaValue(result)
	assert.True(t, res.ToBoolean(), "expected reduced motion setting to be 'reduce'")
}

func TestPageContent(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	content := `<!DOCTYPE html><html><head></head><body><h1>Hello</h1></body></html>`
	p.SetContent(content, nil)

	assert.Equal(t, content, p.Content())
}

func TestPageEvaluate(t *testing.T) {
	t.Parallel()

	t.Run("ok/func_arg", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)

		got := p.Evaluate(
			tb.toGojaValue("(v) => { window.v = v; return window.v }"),
			tb.toGojaValue("test"),
		)

		require.IsType(t, tb.toGojaValue(""), got)
		gotVal := tb.asGojaValue(got)
		assert.Equal(t, "test", gotVal.Export())
	})

	t.Run("ok/void_func", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		h, err := p.EvaluateHandle(tb.toGojaValue(`() => console.log("hello")`))
		assert.Nil(t, h, "expected nil handle")
		assert.Error(t, err)
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name, js, errMsg string
		}{
			{
				"promise",
				`async () => { return await new Promise((res, rej) => { rej('rejected'); }); }`,
				"evaluating JS: rejected",
			},
			{
				"syntax", `() => {`,
				"evaluating JS: SyntaxError: Unexpected token ')'",
			},
			{"undef", "undef", "evaluating JS: ReferenceError: undef is not defined"},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				tb := newTestBrowser(t)
				assertExceptionContains(t, tb.runtime(), func() {
					p := tb.NewPage(nil)
					p.Evaluate(tb.toGojaValue(tc.js))
				}, tc.errMsg)
			})
		}
	})
}

func TestPageGoto(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)

	url := b.staticURL("empty.html")
	r, err := p.Goto(url, nil)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, url, r.URL(), `expected URL to be %q, result of navigation was %q`, url, r.URL())
}

func TestPageGotoDataURI(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t)
	p := b.NewPage(nil)

	r, err := p.Goto("data:text/html,hello", nil)
	require.NoError(t, err)
	assert.Nil(t, r, `expected response to be nil`)
	require.NoError(t, err)
}

func TestPageGotoWaitUntilLoad(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)

	opts := b.toGojaValue(struct {
		WaitUntil string `js:"waitUntil"`
	}{
		WaitUntil: "load",
	})
	_, err := p.Goto(b.staticURL("wait_until.html"), opts)
	require.NoError(t, err)
	var (
		results = p.Evaluate(b.toGojaValue("() => window.results"))
		actual  []string
	)
	_ = b.runtime().ExportTo(b.asGojaValue(results), &actual)

	assert.EqualValues(t,
		[]string{"DOMContentLoaded", "load"}, actual,
		`expected "load" event to have fired`,
	)
}

func TestPageGotoWaitUntilDOMContentLoaded(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)

	opts := b.toGojaValue(struct {
		WaitUntil string `js:"waitUntil"`
	}{
		WaitUntil: "domcontentloaded",
	})
	_, err := p.Goto(b.staticURL("wait_until.html"), opts)
	require.NoError(t, err)
	var (
		results = p.Evaluate(b.toGojaValue("() => window.results"))
		actual  []string
	)
	_ = b.runtime().ExportTo(b.asGojaValue(results), &actual)

	assert.EqualValues(t,
		"DOMContentLoaded", actual[0],
		`expected "DOMContentLoaded" event to have fired`,
	)
}

func TestPageInnerHTML(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		p := newTestBrowser(t).NewPage(nil)
		p.SetContent(sampleHTML, nil)
		assert.Equal(t, `<b>Test</b><ol><li><i>One</i></li></ol>`, p.InnerHTML("div", nil))
	})

	t.Run("err_empty_selector", func(t *testing.T) {
		t.Parallel()

		defer func() {
		}()

		tb := newTestBrowser(t)
		assertExceptionContains(t, tb.runtime(), func() {
			p := tb.NewPage(nil)
			p.InnerHTML("", nil)
		}, "The provided selector is empty")
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		p.SetContent(sampleHTML, nil)
		require.Panics(t, func() { p.InnerHTML("p", tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})) })
	})
}

func TestPageInnerText(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		p := newTestBrowser(t).NewPage(nil)
		p.SetContent(sampleHTML, nil)
		assert.Equal(t, "Test\nOne", p.InnerText("div", nil))
	})

	t.Run("err_empty_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		assertExceptionContains(t, tb.runtime(), func() {
			p := tb.NewPage(nil)
			p.InnerText("", nil)
		}, "The provided selector is empty")
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		p.SetContent(sampleHTML, nil)
		require.Panics(t, func() { p.InnerText("p", tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})) })
	})
}

func TestPageTextContent(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		p := newTestBrowser(t).NewPage(nil)
		p.SetContent(sampleHTML, nil)
		assert.Equal(t, "TestOne", p.TextContent("div", nil))
	})

	t.Run("err_empty_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		assertExceptionContains(t, tb.runtime(), func() {
			p := tb.NewPage(nil)
			p.TextContent("", nil)
		}, "The provided selector is empty")
	})

	t.Run("err_wrong_selector", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		p.SetContent(sampleHTML, nil)
		require.Panics(t, func() { p.TextContent("p", tb.toGojaValue(jsFrameBaseOpts{Timeout: "100"})) })
	})
}

func TestPageInputValue(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	p.SetContent(`
		<input value="hello1">
		<select><option value="hello2" selected></option></select>
		<textarea>hello3</textarea>
     	`, nil)

	got, want := p.InputValue("input", nil), "hello1"
	assert.Equal(t, got, want)

	got, want = p.InputValue("select", nil), "hello2"
	assert.Equal(t, got, want)

	got, want = p.InputValue("textarea", nil), "hello3"
	assert.Equal(t, got, want)
}

// test for: https://github.com/grafana/xk6-browser/issues/132
func TestPageInputSpecialCharacters(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	p.SetContent(`<input id="special">`, nil)
	el, err := p.Query("#special")
	require.NoError(t, err)

	wants := []string{
		"test@k6.io",
		"<hello WoRlD \\/>",
		"{(hello world!)}",
		"!#$%^&*()+_|~±",
		`¯\_(ツ)_/¯`,
	}
	for _, want := range wants {
		el.Fill("", nil)
		el.Type(want, nil)

		got := el.InputValue(nil)
		assert.Equal(t, want, got)
	}
}

func TestPageFill(t *testing.T) {
	// these tests are not parallel by intention because
	// they're testing the same page instance and they're
	// faster when run sequentially.

	p := newTestBrowser(t).NewPage(nil)
	p.SetContent(`
		<input id="text" type="text" value="something" />
		<input id="date" type="date" value="2012-03-12"/>
		<input id="number" type="number" value="42"/>
		<input id="unfillable" type="radio" />
	`, nil)

	happy := []struct{ name, selector, value string }{
		{name: "text", selector: "#text", value: "fill me up"},
		{name: "date", selector: "#date", value: "2012-03-13"},
		{name: "number", selector: "#number", value: "42"},
	}
	sad := []struct{ name, selector, value string }{
		{name: "date", selector: "#date", value: "invalid date"},
		{name: "number", selector: "#number", value: "forty two"},
		{name: "unfillable", selector: "#unfillable", value: "can't touch this"},
	}
	for _, tt := range happy {
		t.Run("happy/"+tt.name, func(t *testing.T) {
			p.Fill(tt.selector, tt.value, nil)
			require.Equal(t, tt.value, p.InputValue(tt.selector, nil))
		})
	}
	for _, tt := range sad {
		t.Run("sad/"+tt.name, func(t *testing.T) {
			require.Panics(t, func() { p.Fill(tt.selector, tt.value, nil) })
		})
	}
}

func TestPageIsChecked(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	p.SetContent(`<input type="checkbox" checked>`, nil)
	assert.True(t, p.IsChecked("input", nil), "expected checkbox to be checked")

	p.SetContent(`<input type="checkbox">`, nil)
	assert.False(t, p.IsChecked("input", nil), "expected checkbox to be unchecked")
}

func TestPageScreenshotFullpage(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.SetViewportSize(tb.toGojaValue(struct {
		Width  float64 `js:"width"`
		Height float64 `js:"height"`
	}{Width: 1280, Height: 800}))
	p.Evaluate(tb.toGojaValue(`
	() => {
		document.body.style.margin = '0';
		document.body.style.padding = '0';
		document.documentElement.style.margin = '0';
		document.documentElement.style.padding = '0';

		const div = document.createElement('div');
		div.style.width = '1280px';
		div.style.height = '800px';
		div.style.background = 'linear-gradient(red, blue)';

		document.body.appendChild(div);
	}
    	`))

	buf := p.Screenshot(tb.toGojaValue(struct {
		FullPage bool `js:"fullPage"`
	}{FullPage: true}))

	reader := bytes.NewReader(buf.Bytes())
	img, err := png.Decode(reader)
	assert.Nil(t, err)

	assert.Equal(t, 1280, img.Bounds().Max.X, "screenshot width is not 1280px as expected, but %dpx", img.Bounds().Max.X)
	assert.Equal(t, 800, img.Bounds().Max.Y, "screenshot height is not 800px as expected, but %dpx", img.Bounds().Max.Y)

	r, _, b, _ := img.At(0, 0).RGBA()
	assert.Greater(t, r, uint32(128))
	assert.Less(t, b, uint32(128))
	r, _, b, _ = img.At(0, 799).RGBA()
	assert.Less(t, r, uint32(128))
	assert.Greater(t, b, uint32(128))
}

func TestPageTitle(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	p.SetContent(`<html><head><title>Some title</title></head></html>`, nil)
	assert.Equal(t, "Some title", p.Title())
}

func TestPageSetExtraHTTPHeaders(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withHTTPServer())

	p := b.NewPage(nil)

	headers := map[string]string{
		"Some-Header": "Some-Value",
	}
	p.SetExtraHTTPHeaders(headers)

	resp, err := p.Goto(b.url("/get"), nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	var body struct{ Headers map[string][]string }
	err = json.Unmarshal(resp.Body().Bytes(), &body)
	require.NoError(t, err)

	h := body.Headers["Some-Header"]
	require.NotEmpty(t, h)
	assert.Equal(t, "Some-Value", h[0])
}

func TestPageWaitForFunction(t *testing.T) {
	t.Parallel()

	// script is here to test we're not getting an error from the
	// waitForFunction call itself and the tests that use it are
	// testing the polling functionality—not the response from
	// waitForFunction.
	script := `
		let resp = page.waitForFunction(%s, %s, %s)
		log('ok: '+resp);`

	t.Run("ok_func_raf_default", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		var log []string
		require.NoError(t, tb.runtime().Set("log", func(s string) { log = append(log, s) }))
		require.NoError(t, tb.runtime().Set("page", p))

		_, err := tb.runJavaScript(`fn = () => {
			if (typeof window._cnt == 'undefined') window._cnt = 0;
			if (window._cnt >= 50) return true;
			window._cnt++;
			return false;
		}`)
		require.NoError(t, err)

		_, err = tb.runJavaScript(script, "fn", "{}", "null")
		require.NoError(t, err)
		assert.Contains(t, log, "ok: null")
	})

	t.Run("ok_func_raf_default_arg", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		require.NoError(t, tb.runtime().Set("page", p))
		var log []string
		require.NoError(t, tb.runtime().Set("log", func(s string) { log = append(log, s) }))

		_, err := tb.runJavaScript(`fn = arg => {
			window._arg = arg;
			return true;
		}`)
		require.NoError(t, err)

		arg := "raf_arg"
		_, err = tb.runJavaScript(script, "fn", "{}", fmt.Sprintf("%q", arg))
		require.NoError(t, err)
		assert.Contains(t, log, "ok: null")

		argEvalJS := p.Evaluate(tb.toGojaValue("() => window._arg"))
		argEval := tb.asGojaValue(argEvalJS)
		var gotArg string
		_ = tb.runtime().ExportTo(argEval, &gotArg)
		assert.Equal(t, arg, gotArg)
	})

	t.Run("ok_func_raf_default_args", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		require.NoError(t, tb.runtime().Set("page", p))
		var log []string
		require.NoError(t, tb.runtime().Set("log", func(s string) { log = append(log, s) }))

		_, err := tb.runJavaScript(`fn = (...args) => {
			window._args = args;
			return true;
		}`)
		require.NoError(t, err)

		args := []int{1, 2, 3}
		argsJS, err := json.Marshal(args)
		require.NoError(t, err)

		_, err = tb.runJavaScript(script, "fn", "{}", fmt.Sprintf("...%s", string(argsJS)))
		require.NoError(t, err)
		assert.Contains(t, log, "ok: null")

		argEvalJS := p.Evaluate(tb.toGojaValue("() => window._args"))
		argEval := tb.asGojaValue(argEvalJS)
		var gotArgs []int
		_ = tb.runtime().ExportTo(argEval, &gotArgs)
		assert.Equal(t, args, gotArgs)
	})

	t.Run("err_expr_raf_timeout", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		rt := tb.vu.Runtime()
		var log []string
		require.NoError(t, rt.Set("log", func(s string) { log = append(log, s) }))
		require.NoError(t, rt.Set("page", p))

		_, err := tb.runJavaScript(script, "false", "{ polling: 'raf', timeout: 500, }", "null")
		require.ErrorContains(t, err, "timed out after 500ms")
	})

	t.Run("err_wrong_polling", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		rt := tb.vu.Runtime()
		require.NoError(t, rt.Set("page", p))

		_, err := tb.runJavaScript(script, "false", "{ polling: 'blah' }", "null")
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			`parsing waitForFunction options: wrong polling option value:`,
			`"blah"; possible values: "raf", "mutation" or number`)
	})

	t.Run("ok_expr_poll_interval", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		require.NoError(t, tb.runtime().Set("page", p))
		var log []string
		require.NoError(t, tb.runtime().Set("log", func(s string) { log = append(log, s) }))

		p.Evaluate(tb.toGojaValue(`() => {
			setTimeout(() => {
				const el = document.createElement('h1');
				el.innerHTML = 'Hello';
				document.body.appendChild(el);
			}, 1000);
		}`))

		script := `
			let resp = page.waitForFunction(%s, %s, %s);
			if (resp) {
				log('ok: '+resp.innerHTML());
			} else {
				log('err: '+err);
			}`

		s := fmt.Sprintf(script, `"document.querySelector('h1')"`, "{ polling: 100, timeout: 2000, }", "null")
		_, err := tb.runJavaScript(s)
		require.NoError(t, err)
		assert.Contains(t, log, "ok: Hello")
	})

	t.Run("ok_func_poll_mutation", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		require.NoError(t, tb.runtime().Set("page", p))
		var log []string
		require.NoError(t, tb.runtime().Set("log", func(s string) { log = append(log, s) }))

		_, err := tb.runJavaScript(`fn = () => document.querySelector('h1') !== null`)
		require.NoError(t, err)

		p.Evaluate(tb.toGojaValue(`() => {
			console.log('calling setTimeout...');
			setTimeout(() => {
				console.log('creating element...');
				const el = document.createElement('h1');
				el.innerHTML = 'Hello';
				document.body.appendChild(el);
			}, 1000);
		}`))

		s := fmt.Sprintf(script, "fn", "{ polling: 'mutation', timeout: 2000, }", "null")
		_, err = tb.runJavaScript(s)
		require.NoError(t, err)
		assert.Contains(t, log, "ok: null")
	})
}

func TestPageWaitForLoadState(t *testing.T) {
	t.Parallel()

	t.Run("err_wrong_event", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		assertExceptionContains(t, tb.runtime(), func() {
			p := tb.NewPage(nil)
			p.WaitForLoadState("none", nil)
		}, `invalid lifecycle event: "none"; must be one of: load, domcontentloaded, networkidle`)
	})
}

// See: The issue #187 for details.
func TestPageWaitForNavigationErrOnCtxDone(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t)
	p := b.NewPage(nil)
	go b.cancelContext()
	<-b.context().Done()
	_, err := p.WaitForNavigation(nil)
	require.ErrorContains(t, err, "canceled")
}

func TestPagePress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)

	p := tb.NewPage(nil)

	p.SetContent(`<input id="text1">`, nil)

	p.Press("#text1", "Shift+KeyA", nil)
	p.Press("#text1", "KeyB", nil)
	p.Press("#text1", "Shift+KeyC", nil)

	require.Equal(t, "AbC", p.InputValue("#text1", nil))
}

func TestPageURL(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withHTTPServer())

	p := b.NewPage(nil)
	assert.Equal(t, common.BlankPage, p.URL())

	resp, err := p.Goto(b.url("/get"), nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Regexp(t, "http://.*/get", p.URL())
}

func TestPageClose(t *testing.T) {
	t.Parallel()

	t.Run("page_from_browser", func(t *testing.T) {
		t.Parallel()

		b := newTestBrowser(t, withHTTPServer())

		p := b.NewPage(nil)

		err := p.Close(nil)
		assert.NoError(t, err)
	})

	t.Run("page_from_browserContext", func(t *testing.T) {
		t.Parallel()

		b := newTestBrowser(t, withHTTPServer())

		c, err := b.NewContext(nil)
		require.NoError(t, err)
		p, err := c.NewPage()
		require.NoError(t, err)

		err = p.Close(nil)
		assert.NoError(t, err)
	})
}

func TestPageOn(t *testing.T) { //nolint:gocognit
	t.Parallel()

	const blankPage = "about:blank"

	testCases := []struct {
		name      string
		consoleFn string
		assertFn  func(*common.ConsoleMessage) bool
	}{
		{
			name:      "on console.log",
			consoleFn: "() => console.log('this is a log message')",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "log" &&
					cm.Text == "this is a log message" &&
					cm.Args[0].JSONValue().String() == "this is a log message" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.debug",
			consoleFn: "() => console.debug('this is a debug message')",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "debug" &&
					cm.Text == "this is a debug message" &&
					cm.Args[0].JSONValue().String() == "this is a debug message" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.info",
			consoleFn: "() => console.info('this is an info message')",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "info" &&
					cm.Text == "this is an info message" &&
					cm.Args[0].JSONValue().String() == "this is an info message" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.error",
			consoleFn: "() => console.error('this is an error message')",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "error" &&
					cm.Text == "this is an error message" &&
					cm.Args[0].JSONValue().String() == "this is an error message" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.warn",
			consoleFn: "() => console.warn('this is a warning message')",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "warning" &&
					cm.Text == "this is a warning message" &&
					cm.Args[0].JSONValue().String() == "this is a warning message" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.dir",
			consoleFn: "() => console.dir(document.location)",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "dir" &&
					cm.Text == "Location" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.dirxml",
			consoleFn: "() => console.dirxml(document.location)",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "dirxml" &&
					cm.Text == "Location" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.table",
			consoleFn: "() => console.table([['Grafana', 'k6'], ['Grafana', 'Mimir']])",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "table" &&
					cm.Text == "Array(2)" &&
					cm.Args[0].JSONValue().String() == "Grafana,k6,Grafana,Mimir" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.trace",
			consoleFn: "() => console.trace('trace example')",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "trace" &&
					cm.Text == "trace example" &&
					cm.Args[0].JSONValue().String() == "trace example" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.clear",
			consoleFn: "() => console.clear()",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "clear" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.group",
			consoleFn: "() => console.group()",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "startGroup" &&
					cm.Text == "console.group" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.groupCollapsed",
			consoleFn: "() => console.groupCollapsed()",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "startGroupCollapsed" &&
					cm.Text == "console.groupCollapsed" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.groupEnd",
			consoleFn: "() => console.groupEnd()",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "endGroup" &&
					cm.Text == "console.groupEnd" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.assert",
			consoleFn: "() => console.assert(2 == 3)", // Only writes to console if assertion is false
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "assert" &&
					cm.Text == "console.assert" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.count (default label)",
			consoleFn: "() => console.count()", // default label
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "count" &&
					cm.Text == "default: 1" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.count",
			consoleFn: "() => console.count('k6')",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "count" &&
					cm.Text == "k6: 1" &&
					cm.Page.URL() == blankPage
			},
		},
		{
			name:      "on console.time",
			consoleFn: "() => { console.time('k6'); console.timeEnd('k6'); }",
			assertFn: func(cm *common.ConsoleMessage) bool {
				return cm.Type == "timeEnd" && strings.HasPrefix(cm.Text, "k6: 0.0") &&
					cm.Page.URL() == blankPage
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Use withSkipClose() opt as we will close it
			// manually to force the page.TaskQueue closing
			tb := newTestBrowser(t)
			p := tb.NewPage(nil)

			var (
				assertOne, assertTwo bool
				done1                = make(chan bool)
				done2                = make(chan bool)

				assertTO bool
				testTO   = 2500 * time.Millisecond
			)

			// Console Messages should be multiplexed for every registered handler
			eventHandlerOne := func(cm *common.ConsoleMessage) {
				defer close(done1)
				assertOne = tc.assertFn(cm)
			}

			eventHandlerTwo := func(cm *common.ConsoleMessage) {
				defer close(done2)
				assertTwo = tc.assertFn(cm)
			}

			// eventHandlerOne and eventHandlerTwo will be called from a
			// separate goroutine from within the page's async event loop.
			// This is why we need to wait on done1 and done2 to be closed.
			err := p.On("console", eventHandlerOne)
			require.NoError(t, err)

			err = p.On("console", eventHandlerTwo)
			require.NoError(t, err)

			p.Evaluate(tb.toGojaValue(tc.consoleFn))

			select {
			case <-done1:
			case <-time.After(testTO):
				assertTO = true
			}

			select {
			case <-done2:
			case <-time.After(testTO):
				assertTO = true
			}

			assert.False(t, assertTO, "test timed out before event handlers were called")
			assert.True(t, assertOne, "error asserting console message for assertOne")
			assert.True(t, assertTwo, "error asserting console message for assertTwo")
		})
	}
}

func assertExceptionContains(t *testing.T, rt *goja.Runtime, fn func(), expErrMsg string) {
	t.Helper()

	cal, _ := goja.AssertFunction(rt.ToValue(fn))

	_, err := cal(goja.Undefined())
	require.ErrorContains(t, err, expErrMsg)
}

func TestPageTimeout(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                     string
		defaultTimeout           time.Duration
		defaultNavigationTimeout time.Duration
	}{
		{
			name:           "fail when timeout exceeds default timeout",
			defaultTimeout: 1 * time.Millisecond,
		},
		{
			name:                     "fail when timeout exceeds default navigation timeout",
			defaultNavigationTimeout: 1 * time.Millisecond,
		},
		{
			name:                     "default navigation timeout supersedes default timeout",
			defaultTimeout:           30 * time.Second,
			defaultNavigationTimeout: 1 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withHTTPServer())

			tb.withHandler("/slow", func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(100 * time.Millisecond)
				fmt.Fprintf(w, `sorry for being so slow`)
			})

			p := tb.NewPage(nil)

			if tc.defaultTimeout != 0 {
				p.SetDefaultTimeout(tc.defaultTimeout.Milliseconds())
			}
			if tc.defaultNavigationTimeout != 0 {
				p.SetDefaultNavigationTimeout(tc.defaultNavigationTimeout.Milliseconds())
			}

			res, err := p.Goto(tb.url("/slow"), nil)
			require.Nil(t, res)
			assert.ErrorContains(t, err, "timed out after")
		})
	}
}

func TestPageWaitForSelector(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		url       string
		opts      map[string]any
		selector  string
		errAssert func(*testing.T, error)
	}{
		{
			name:     "should wait for selector",
			url:      "wait_for.html",
			selector: "#my-div",
			errAssert: func(t *testing.T, e error) {
				t.Helper()
				assert.Nil(t, e)
			},
		},
		{
			name: "should TO waiting for selector",
			url:  "wait_for.html",
			opts: map[string]any{
				// set a timeout smaller than the time
				// it takes the element to show up
				"timeout": "50",
			},
			selector: "#my-div",
			errAssert: func(t *testing.T, e error) {
				t.Helper()
				assert.ErrorContains(t, e, "timed out after")
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())

			page := tb.NewPage(nil)
			_, err := page.Goto(tb.staticURL(tc.url), nil)
			require.NoError(t, err)

			_, err = page.WaitForSelector(tc.selector, tb.toGojaValue(tc.opts))
			tc.errAssert(t, err)
		})
	}
}

func TestPageThrottleNetwork(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                     string
		networkProfile           common.NetworkProfile
		wantMinRoundTripDuration int64
	}{
		{
			name: "none",
			networkProfile: common.NetworkProfile{
				Latency:  0,
				Download: -1,
				Upload:   -1,
			},
		},
		{
			// In the ping.html file, an async ping request is made. The time it takes
			// to perform the roundtrip of calling ping and getting the response is
			// measured and used to assert that Latency has been correctly used.
			name: "latency",
			networkProfile: common.NetworkProfile{
				Latency:  100,
				Download: -1,
				Upload:   -1,
			},
			wantMinRoundTripDuration: 100,
		},
		{
			// In the ping.html file, an async ping request is made, the ping response
			// returns the request body (around a 1MB). The time it takes to perform the
			// roundtrip of calling ping and getting the response body is measured and
			// used to assert that Download has been correctly used.
			name: "download",
			networkProfile: common.NetworkProfile{
				Latency:  0,
				Download: 1000,
				Upload:   -1,
			},
			wantMinRoundTripDuration: 1000,
		},
		{
			// In the ping.html file, an async ping request is made with around a 1MB body.
			// The time it takes to perform the roundtrip of calling ping is measured
			// and used to assert that Upload has been correctly used.
			name: "upload",
			networkProfile: common.NetworkProfile{
				Latency:  0,
				Download: -1,
				Upload:   1000,
			},
			wantMinRoundTripDuration: 1000,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())

			tb.withHandler("/ping", func(w http.ResponseWriter, req *http.Request) {
				defer func() {
					err := req.Body.Close()
					require.NoError(t, err)
				}()
				bb, err := io.ReadAll(req.Body)
				require.NoError(t, err)

				fmt.Fprint(w, string(bb))
			})

			page := tb.NewPage(nil)

			err := page.ThrottleNetwork(tc.networkProfile)
			require.NoError(t, err)

			_, err = page.Goto(tb.staticURL("ping.html"), nil)
			require.NoError(t, err)

			selector := `div[id="result"]`

			// result selector only appears once the page gets a response
			// from the async ping request.
			_, err = page.WaitForSelector(selector, nil)
			require.NoError(t, err)

			resp := page.InnerText(selector, nil)
			ms, err := strconv.ParseInt(resp, 10, 64)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, ms, tc.wantMinRoundTripDuration)
		})
	}
}

// This test will first navigate to the ping.html site a few times, record the
// average time it takes to run the test. Next it will repeat the steps but
// first apply CPU throttling. The average duration with CPU throttling
// enabled should be longer than without it.
func TestPageThrottleCPU(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())

	tb.withHandler("/ping", func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			err := req.Body.Close()
			require.NoError(t, err)
		}()
		bb, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		fmt.Fprint(w, string(bb))
	})

	page := tb.NewPage(nil)
	const iterations = 5

	noCPUThrottle := performPingTest(t, tb, page, iterations)

	err := page.ThrottleCPU(common.CPUProfile{
		Rate: 50,
	})
	require.NoError(t, err)

	withCPUThrottle := performPingTest(t, tb, page, iterations)

	assert.Greater(t, withCPUThrottle, noCPUThrottle)
}

func performPingTest(t *testing.T, tb *testBrowser, page *common.Page, iterations int) int64 {
	t.Helper()

	var ms int64
	for i := 0; i < iterations; i++ {
		start := time.Now()

		_, err := page.Goto(tb.staticURL("ping.html"), nil)
		require.NoError(t, err)

		selector := `div[id="result"]`

		// result selector only appears once the page gets a response
		// from the async ping request.
		_, err = page.WaitForSelector(selector, nil)
		require.NoError(t, err)

		ms += time.Since(start).Abs().Milliseconds()
	}

	return ms / int64(iterations)
}
