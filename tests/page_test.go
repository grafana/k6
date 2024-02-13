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
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/browser"
	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext/k6test"
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

	tb := newTestBrowser(t,
		withFileServer(),
	)
	defer tb.Browser.Close()

	page := tb.NewPage(nil)
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := page.Goto(
		tb.staticURL("iframe_test_main.html"),
		opts,
	)
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

	err = button1Handle.Click(common.NewElementHandleClickOptions(button1Handle.Timeout()))
	assert.Nil(t, err)

	v := frame2.Evaluate(`() => window.buttonClicked`)
	bv := asBool(t, v)
	assert.True(t, bv, "button hasn't been clicked")
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

	result := p.Evaluate(`() => matchMedia('print').matches`)
	assert.IsTypef(t, true, result, "expected media 'print'")

	result = p.Evaluate(`() => matchMedia('(prefers-color-scheme: dark)').matches`)
	assert.IsTypef(t, true, result, "expected color scheme 'dark'")

	result = p.Evaluate(`() => matchMedia('(prefers-reduced-motion: reduce)').matches`)
	assert.IsTypef(t, true, result, "expected reduced motion setting to be 'reduce'")
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
			`(v) => { window.v = v; return window.v }`,
			"test",
		)
		s := asString(t, got)
		assert.Equal(t, "test", s)
	})

	t.Run("ok/void_func", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		h, err := p.EvaluateHandle(`() => console.log("hello")`)
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
					p.Evaluate(tc.js)
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
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	r, err := p.Goto(
		url,
		opts,
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, url, r.URL(), `expected URL to be %q, result of navigation was %q`, url, r.URL())
}

func TestPageGotoDataURI(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t)
	p := b.NewPage(nil)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	r, err := p.Goto(
		"data:text/html,hello",
		opts,
	)
	require.NoError(t, err)
	assert.Nil(t, r, `expected response to be nil`)
	require.NoError(t, err)
}

func TestPageGotoWaitUntilLoad(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)

	opts := &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventLoad,
		Timeout:   common.DefaultTimeout,
	}
	_, err := p.Goto(b.staticURL("wait_until.html"), opts)
	require.NoError(t, err)
	results := p.Evaluate(`() => window.results`)
	assert.EqualValues(t,
		[]any{"DOMContentLoaded", "load"}, results,
		`expected "load" event to have fired`,
	)
}

func TestPageGotoWaitUntilDOMContentLoaded(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)

	opts := &common.FrameGotoOptions{
		WaitUntil: common.LifecycleEventDOMContentLoad,
		Timeout:   common.DefaultTimeout,
	}
	_, err := p.Goto(b.staticURL("wait_until.html"), opts)
	require.NoError(t, err)
	v := p.Evaluate(`() => window.results`)
	results, ok := v.([]any)
	if !ok {
		t.Fatalf("expected results to be a slice, got %T", v)
	}
	require.True(t, len(results) >= 1, "expected at least one result")
	assert.EqualValues(t,
		"DOMContentLoaded", results[0],
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
	}{
		Width: 1280, Height: 800,
	}))
	p.Evaluate(`
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
    `)

	opts := common.NewPageScreenshotOptions()
	opts.FullPage = true
	buf, err := p.Screenshot(opts, &mockPersister{})
	require.NoError(t, err)

	reader := bytes.NewReader(buf)
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

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	resp, err := p.Goto(
		b.url("/get"),
		opts,
	)
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
		var page;
		const test = async function() {
			page = browser.newPage();
			let resp = await page.waitForFunction(%s, %s, %s);
			log('ok: '+resp);
		}
		test();
		`

	t.Run("ok_func_raf_default", func(t *testing.T) {
		t.Parallel()

		vu, rt, log, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := rt.RunString(`fn = () => {
			if (typeof window._cnt == 'undefined') window._cnt = 0;
			if (window._cnt >= 50) return true;
			window._cnt++;
			return false;
		}`)
		require.NoError(t, err)

		_, err = vu.TestRT.RunOnEventLoop(fmt.Sprintf(script, "fn", "{}", "null"))
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: null")
	})

	t.Run("ok_func_raf_default_arg", func(t *testing.T) {
		t.Parallel()

		vu, rt, log, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := rt.RunString(`fn = arg => {
			window._arg = arg;
			return true;
		}`)
		require.NoError(t, err)

		arg := "raf_arg"
		_, err = vu.TestRT.RunOnEventLoop(fmt.Sprintf(script, "fn", "{}", fmt.Sprintf("%q", arg)))
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: null")

		argEval, err := rt.RunString(`page.evaluate(() => window._arg);`)
		require.NoError(t, err)

		var gotArg string
		_ = rt.ExportTo(argEval, &gotArg)
		assert.Equal(t, arg, gotArg)
	})

	t.Run("ok_func_raf_default_args", func(t *testing.T) {
		t.Parallel()

		vu, rt, log, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := rt.RunString(`fn = (...args) => {
			window._args = args;
			return true;
		}`)
		require.NoError(t, err)

		args := []int{1, 2, 3}
		argsJS, err := json.Marshal(args)
		require.NoError(t, err)

		_, err = vu.TestRT.RunOnEventLoop(fmt.Sprintf(script, "fn", "{}", fmt.Sprintf("...%s", string(argsJS))))
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: null")

		argEval, err := rt.RunString(`page.evaluate(() => window._args);`)
		require.NoError(t, err)

		var gotArgs []int
		_ = rt.ExportTo(argEval, &gotArgs)
		assert.Equal(t, args, gotArgs)
	})

	t.Run("err_expr_raf_timeout", func(t *testing.T) {
		t.Parallel()

		vu, _, _, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := vu.TestRT.RunOnEventLoop(fmt.Sprintf(script, "false", "{ polling: 'raf', timeout: 500, }", "null"))
		require.ErrorContains(t, err, "timed out after 500ms")
	})

	t.Run("err_wrong_polling", func(t *testing.T) {
		t.Parallel()

		vu, _, _, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := vu.TestRT.RunOnEventLoop(fmt.Sprintf(script, "false", "{ polling: 'blah' }", "null"))
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			`parsing waitForFunction options: wrong polling option value:`,
			`"blah"; possible values: "raf", "mutation" or number`)
	})

	t.Run("ok_expr_poll_interval", func(t *testing.T) {
		t.Parallel()

		vu, rt, log, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := rt.RunString(`
		const page = browser.newPage();
		page.evaluate(() => {
			setTimeout(() => {
				const el = document.createElement('h1');
				el.innerHTML = 'Hello';
				document.body.appendChild(el);
			}, 1000);
		});`)
		require.NoError(t, err)

		script := `
		const test = async function() {
			let resp = await page.waitForFunction(%s, %s, %s);
			if (resp) {
				log('ok: '+resp.innerHTML());
			} else {
				log('err: '+err);
			}
		}
		test();`

		s := fmt.Sprintf(script, `"document.querySelector('h1')"`, "{ polling: 100, timeout: 2000, }", "null")
		_, err = vu.TestRT.RunOnEventLoop(s)
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: Hello")
	})

	t.Run("ok_func_poll_mutation", func(t *testing.T) {
		t.Parallel()

		vu, rt, log, cleanUp := startIteration(t)
		defer cleanUp()

		_, err := rt.RunString(`
		fn = () => document.querySelector('h1') !== null

		const page = browser.newPage();
		page.evaluate(() => {
			console.log('calling setTimeout...');
			setTimeout(() => {
				console.log('creating element...');
				const el = document.createElement('h1');
				el.innerHTML = 'Hello';
				document.body.appendChild(el);
			}, 1000);
		})`)
		require.NoError(t, err)

		script := `
		const test = async function() {
			let resp = await page.waitForFunction(%s, %s, %s);
			log('ok: '+resp);
		}
		test();`

		s := fmt.Sprintf(script, "fn", "{ polling: 'mutation', timeout: 2000, }", "null")
		_, err = vu.TestRT.RunOnEventLoop(s)
		require.NoError(t, err)
		assert.Contains(t, *log, "ok: null")
	})
}

// startIteration will work with the event system to start chrome and
// (more importantly) set browser as the mapped browser instance which will
// force all tests that work with this to go through the mapping layer.
// This returns a cleanup function which should be deferred.
func startIteration(t *testing.T) (*k6test.VU, *goja.Runtime, *[]string, func()) {
	t.Helper()

	vu := k6test.NewVU(t)
	rt := vu.Runtime()

	mod := browser.New().NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)

	// Setting the mapped browser into the vu's goja runtime.
	require.NoError(t, rt.Set("browser", jsMod.Browser))

	// Setting log, which is used by the callers to assert that certain actions
	// have been made.
	var log []string
	require.NoError(t, rt.Set("log", func(s string) { log = append(log, s) }))

	vu.ActivateVU()
	vu.StartIteration(t)

	return vu, rt, &log, func() { t.Helper(); vu.EndIteration(t) }
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
	_, err := p.WaitForNavigation(
		common.NewFrameWaitForNavigationOptions(p.Timeout()),
	)
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

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	resp, err := p.Goto(
		b.url("/get"),
		opts,
	)
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

func TestPageOn(t *testing.T) {
	t.Parallel()

	const blankPage = "about:blank"

	testCases := []struct {
		name      string
		consoleFn string
		assertFn  func(*testing.T, *common.ConsoleMessage)
	}{
		{
			name:      "on console.log",
			consoleFn: "() => console.log('this is a log message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "log", cm.Type)
				assert.Equal(t, "this is a log message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is a log message", val)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.debug",
			consoleFn: "() => console.debug('this is a debug message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "debug", cm.Type)
				assert.Equal(t, "this is a debug message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is a debug message", val)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.info",
			consoleFn: "() => console.info('this is an info message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "info", cm.Type)
				assert.Equal(t, "this is an info message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is an info message", val)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.error",
			consoleFn: "() => console.error('this is an error message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "error", cm.Type)
				assert.Equal(t, "this is an error message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is an error message", val)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.warn",
			consoleFn: "() => console.warn('this is a warning message')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "warning", cm.Type)
				assert.Equal(t, "this is a warning message", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "this is a warning message", val)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.dir",
			consoleFn: "() => console.dir(document.location)",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "dir", cm.Type)
				assert.Equal(t, "Location", cm.Text)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.dirxml",
			consoleFn: "() => console.dirxml(document.location)",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "dirxml", cm.Type)
				assert.Equal(t, "Location", cm.Text)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.table",
			consoleFn: "() => console.table([['Grafana', 'k6'], ['Grafana', 'Mimir']])",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "table", cm.Type)
				assert.Equal(t, "Array(2)", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, `[["Grafana","k6"],["Grafana","Mimir"]]`, val)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.trace",
			consoleFn: "() => console.trace('trace example')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "trace", cm.Type)
				assert.Equal(t, "trace example", cm.Text)
				val, err := cm.Args[0].JSONValue()
				assert.NoError(t, err)
				assert.Equal(t, "trace example", val)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.clear",
			consoleFn: "() => console.clear()",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "clear", cm.Type)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.group",
			consoleFn: "() => console.group()",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "startGroup", cm.Type)
				assert.Equal(t, "console.group", cm.Text)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.groupCollapsed",
			consoleFn: "() => console.groupCollapsed()",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "startGroupCollapsed", cm.Type)
				assert.Equal(t, "console.groupCollapsed", cm.Text)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.groupEnd",
			consoleFn: "() => console.groupEnd()",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "endGroup", cm.Type)
				assert.Equal(t, "console.groupEnd", cm.Text)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.assert",
			consoleFn: "() => console.assert(2 == 3)", // Only writes to console if assertion is false
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "assert", cm.Type)
				assert.Equal(t, "console.assert", cm.Text)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.count (default label)",
			consoleFn: "() => console.count()", // default label
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "count", cm.Type)
				assert.Equal(t, "default: 1", cm.Text)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.count",
			consoleFn: "() => console.count('k6')",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "count", cm.Type)
				assert.Equal(t, "k6: 1", cm.Text)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
			},
		},
		{
			name:      "on console.time",
			consoleFn: "() => { console.time('k6'); console.timeEnd('k6'); }",
			assertFn: func(t *testing.T, cm *common.ConsoleMessage) {
				t.Helper()
				assert.Equal(t, "timeEnd", cm.Type)
				assert.Regexp(t, `^k6: [0-9]+\.[0-9]+`, cm.Text, `expected prefix "k6: <a float>" but got %q`, cm.Text)
				assert.True(t, cm.Page.URL() == blankPage, "url is not %s", blankPage)
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
				done1 = make(chan bool)
				done2 = make(chan bool)

				testTO = 2500 * time.Millisecond
			)

			// Console Messages should be multiplexed for every registered handler
			eventHandlerOne := func(cm *common.ConsoleMessage) {
				defer close(done1)
				tc.assertFn(t, cm)
			}

			eventHandlerTwo := func(cm *common.ConsoleMessage) {
				defer close(done2)
				tc.assertFn(t, cm)
			}

			// eventHandlerOne and eventHandlerTwo will be called from a
			// separate goroutine from within the page's async event loop.
			// This is why we need to wait on done1 and done2 to be closed.
			err := p.On("console", eventHandlerOne)
			require.NoError(t, err)

			err = p.On("console", eventHandlerTwo)
			require.NoError(t, err)

			p.Evaluate(tc.consoleFn)

			select {
			case <-done1:
			case <-time.After(testTO):
				assert.Fail(t, "test timed out before eventHandlerOne completed")
			}

			select {
			case <-done2:
			case <-time.After(testTO):
				assert.Fail(t, "test timed out before eventHandlerTwo completed")
			}
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

			var timeout time.Duration
			if tc.defaultTimeout != 0 {
				timeout = tc.defaultTimeout
				p.SetDefaultTimeout(tc.defaultTimeout.Milliseconds())
			}
			if tc.defaultNavigationTimeout != 0 {
				timeout = tc.defaultNavigationTimeout
				p.SetDefaultNavigationTimeout(tc.defaultNavigationTimeout.Milliseconds())
			}

			opts := &common.FrameGotoOptions{
				Timeout: timeout,
			}
			res, err := p.Goto(
				tb.url("/slow"),
				opts,
			)
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
			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := page.Goto(
				tb.staticURL(tc.url),
				opts,
			)
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

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err = page.Goto(
				tb.staticURL("ping.html"),
				opts,
			)
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

		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		_, err := page.Goto(
			tb.staticURL("ping.html"),
			opts,
		)
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

func TestPageIsVisible(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		selector string
		options  common.FrameIsVisibleOptions
		want     bool
		wantErr  string
	}{
		{
			name:     "visible",
			selector: "div[id=my-div]",
			want:     true,
		},
		{
			name:     "not_visible",
			selector: "div[id=my-div-3]",
			want:     false,
		},
		{
			name:     "not_found",
			selector: "div[id=does-not-exist]",
			want:     false,
		},
		{
			name:     "first_div",
			selector: "div",
			want:     true,
		},
		{
			name:     "first_div",
			selector: "div",
			options: common.FrameIsVisibleOptions{
				Strict: true,
			},
			wantErr: "error:strictmodeviolation",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())

			page := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := page.Goto(
				tb.staticURL("visible.html"),
				opts,
			)
			require.NoError(t, err)

			got, err := page.IsVisible(tc.selector, tb.toGojaValue(tc.options))

			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPageIsHidden(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		selector string
		options  common.FrameIsVisibleOptions
		want     bool
		wantErr  string
	}{
		{
			name:     "hidden",
			selector: "div[id=my-div-3]",
			want:     true,
		},
		{
			name:     "visible",
			selector: "div[id=my-div]",
			want:     false,
		},
		{
			name:     "not_found",
			selector: "div[id=does-not-exist]",
			want:     true,
		},
		{
			name:     "first_div",
			selector: "div",
			want:     false,
		},
		{
			name:     "first_div",
			selector: "div",
			options: common.FrameIsVisibleOptions{
				Strict: true,
			},
			wantErr: "error:strictmodeviolation",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := newTestBrowser(t, withFileServer())

			page := tb.NewPage(nil)

			opts := &common.FrameGotoOptions{
				Timeout: common.DefaultTimeout,
			}
			_, err := page.Goto(
				tb.staticURL("visible.html"),
				opts,
			)
			require.NoError(t, err)

			got, err := page.IsHidden(tc.selector, tb.toGojaValue(tc.options))

			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
