package tests

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"testing"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed static/mouse_helper.js
var mouseHelperScriptSource string

//nolint:gochecknoglobals
var htmlInputButton = fmt.Sprintf(`
<!DOCTYPE html>
<html>
  <head>
	<title>Button test</title>
  </head>
  <body>
	<script>%s</script>
	<button>Click target</button>
	<script>
	  window.result = 'Was not clicked';
	  window.offsetX = undefined;
	  window.offsetY = undefined;
	  window.pageX = undefined;
	  window.pageY = undefined;
	  window.shiftKey = undefined;
	  window.pageX = undefined;
	  window.pageY = undefined;
	  window.bubbles = undefined;
	  document.querySelector('button').addEventListener('click', e => {
		result = 'Clicked';
		offsetX = e.offsetX;
		offsetY = e.offsetY;
		pageX = e.pageX;
		pageY = e.pageY;
		shiftKey = e.shiftKey;
		bubbles = e.bubbles;
		cancelable = e.cancelable;
		composed = e.composed;
	  }, false);
	</script>
  </body>
</html>
`, mouseHelperScriptSource)

func TestElementHandleBoundingBoxInvisibleElement(t *testing.T) {
	p := newTestBrowser(t).NewPage(nil)

	p.SetContent(`<div style="display:none">hello</div>`, nil)
	element := p.Query("div")

	require.Nil(t, element.BoundingBox())
}

func TestElementHandleBoundingBoxSVG(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.SetContent(`
		<svg xmlns="http://www.w3.org/2000/svg" width="500" height="500">
			<rect id="theRect" x="30" y="50" width="200" height="300"></rect>
		</svg>
	`, nil)
	element := p.Query("#therect")
	bbox := element.BoundingBox()
	pageFn := `e => {
        const rect = e.getBoundingClientRect();
        return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
    }`
	var r api.Rect
	webBbox := p.Evaluate(tb.toGojaValue(pageFn), tb.toGojaValue(element))
	wb, _ := webBbox.(goja.Value)
	err := tb.runtime().ExportTo(wb, &r)
	require.NoError(t, err)

	require.EqualValues(t, bbox, &r)
}

func TestElementHandleClick(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.SetContent(htmlInputButton, nil)

	button := p.Query("button")
	err := tb.await(func() error {
		_ = button.Click(tb.toGojaValue(struct {
			NoWaitAfter bool `js:"noWaitAfter"`
		}{
			// FIX: this is just a workaround because navigation is never triggered
			// and we'd be waiting for it to happen otherwise!
			NoWaitAfter: true,
		}))
		return nil
	})
	require.NoError(t, err)

	res := tb.asGojaValue(p.Evaluate(tb.toGojaValue("() => window['result']")))
	assert.Equal(t, res.String(), "Clicked")
}

func TestElementHandleClickWithNodeRemoved(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.SetContent(htmlInputButton, nil)

	// Remove all nodes
	p.Evaluate(tb.toGojaValue("() => delete window['Node']"))

	button := p.Query("button")
	err := tb.await(func() error {
		_ = button.Click(tb.toGojaValue(struct {
			NoWaitAfter bool `js:"noWaitAfter"`
		}{
			// FIX: this is just a workaround because navigation is never triggered
			// and we'd be waiting for it to happen otherwise!
			NoWaitAfter: true,
		}))
		return nil
	})
	require.NoError(t, err)

	res := tb.asGojaValue(p.Evaluate(tb.toGojaValue("() => window['result']")))
	assert.Equal(t, res.String(), "Clicked")
}

func TestElementHandleClickWithDetachedNode(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.SetContent(htmlInputButton, nil)
	button := p.Query("button")

	// Detach node to panic when clicked
	p.Evaluate(tb.toGojaValue("button => button.remove()"), tb.toGojaValue(button))

	err := tb.await(func() error {
		_ = button.Click(tb.toGojaValue(struct {
			NoWaitAfter bool `js:"noWaitAfter"`
		}{
			// FIX: this is just a workaround because navigation is never triggered and we'd be waiting for
			// it to happen otherwise!
			NoWaitAfter: true,
		}))
		return nil
	})
	assert.ErrorContains(
		t, err,
		"element is not attached to the DOM",
		"expected click to result in correct error to panic",
	)
}

func TestElementHandleClickConcealedLink(t *testing.T) {
	const (
		wantBefore = "🙈"
		wantAfter  = "🐵"
	)

	tb := newTestBrowser(t, withFileServer())
	p := tb.NewContext(
		tb.toGojaValue(struct {
			Viewport common.Viewport `js:"viewport"`
		}{
			Viewport: common.Viewport{
				Width:  500,
				Height: 240,
			},
		}),
	).NewPage()

	clickResult := func() string {
		const cmd = `
			() => window.clickResult
		`
		cr := p.Evaluate(tb.toGojaValue(cmd))
		return tb.asGojaValue(cr).String()
	}
	require.NotNil(t, p.Goto(tb.staticURL("/concealed_link.html"), nil))
	require.Equal(t, wantBefore, clickResult())

	err := tb.await(func() error {
		_ = p.Click("#concealed", nil)
		return nil
	})
	require.NoError(t, err, "element should be clickable")
	require.Equal(t, wantAfter, clickResult())
}

func TestElementHandleNonClickable(t *testing.T) {
	tb := newTestBrowser(t, withFileServer())
	p := tb.NewContext(nil).NewPage()

	require.NotNil(t, p.Goto(tb.staticURL("/non_clickable.html"), nil))
	err := tb.await(func() error {
		_ = p.Click("#non-clickable", nil)
		return nil
	})
	require.Error(t, err, "element should not be clickable")
}

func TestElementHandleGetAttribute(t *testing.T) {
	const want = "https://somewhere"

	p := newTestBrowser(t).NewPage(nil)
	p.SetContent(`
		<a id="dark-mode-toggle-X" href="https://somewhere">Dark</a>
	`, nil)

	el := p.Query("#dark-mode-toggle-X")
	got := el.GetAttribute("href").String()
	assert.Equal(t, want, got)
}

func TestElementHandleInputValue(t *testing.T) {
	p := newTestBrowser(t).NewPage(nil)

	p.SetContent(`
		<input value="hello1">
		<select><option value="hello2" selected></option></select>
		<textarea>hello3</textarea>
    	`, nil)

	element := p.Query("input")
	value := element.InputValue(nil)
	element.Dispose()
	assert.Equal(t, value, "hello1", `expected input value "hello1", got %q`, value)

	element = p.Query("select")
	value = element.InputValue(nil)
	element.Dispose()
	assert.Equal(t, value, "hello2", `expected input value "hello2", got %q`, value)

	element = p.Query("textarea")
	value = element.InputValue(nil)
	element.Dispose()
	assert.Equal(t, value, "hello3", `expected input value "hello3", got %q`, value)
}

func TestElementHandleIsChecked(t *testing.T) {
	p := newTestBrowser(t).NewPage(nil)

	p.SetContent(`<input type="checkbox" checked>`, nil)
	element := p.Query("input")
	assert.True(t, element.IsChecked(), "expected checkbox to be checked")
	element.Dispose()

	p.SetContent(`<input type="checkbox">`, nil)
	element = p.Query("input")
	assert.False(t, element.IsChecked(), "expected checkbox to be unchecked")
	element.Dispose()
}

func TestElementHandleQueryAll(t *testing.T) {
	const (
		wantLiLen = 2
		query     = "li.ali"
	)

	p := newTestBrowser(t).NewPage(nil)
	p.SetContent(`
		<ul id="aul">
			<li class="ali">1</li>
			<li class="ali">2</li>
		</ul>
  	`, nil)

	t.Run("element_handle", func(t *testing.T) {
		assert.Equal(t, wantLiLen, len(p.Query("#aul").QueryAll(query)))
	})
	t.Run("page", func(t *testing.T) {
		assert.Equal(t, wantLiLen, len(p.QueryAll(query)))
	})
	t.Run("frame", func(t *testing.T) {
		assert.Equal(t, wantLiLen, len(p.MainFrame().QueryAll(query)))
	})
}

func TestElementHandleScreenshot(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.SetViewportSize(tb.toGojaValue(struct {
		Width  float64 `js:"width"`
		Height float64 `js:"height"`
	}{Width: 800, Height: 600}))
	p.Evaluate(tb.toGojaValue(`
		() => {
			document.body.style.margin = '0';
			document.body.style.padding = '0';
			document.documentElement.style.margin = '0';
			document.documentElement.style.padding = '0';

			const div = document.createElement('div');
			div.style.marginTop = '400px';
			div.style.marginLeft = '100px';
			div.style.width = '100px';
			div.style.height = '100px';
			div.style.background = 'red';

			document.body.appendChild(div);
		}
    	`))

	elem := p.Query("div")
	buf := elem.Screenshot(nil)

	reader := bytes.NewReader(buf.Bytes())
	img, err := png.Decode(reader)
	assert.Nil(t, err)

	assert.Equal(t, 100, img.Bounds().Max.X, "screenshot width is not 100px as expected, but %dpx", img.Bounds().Max.X)
	assert.Equal(t, 100, img.Bounds().Max.Y, "screenshot height is not 100px as expected, but %dpx", img.Bounds().Max.Y)

	r, g, b, _ := img.At(0, 0).RGBA()
	assert.Equal(t, uint32(255), r>>8) // each color component has been scaled by alpha (<<8)
	assert.Equal(t, uint32(0), g)
	assert.Equal(t, uint32(0), b)
	r, g, b, _ = img.At(99, 99).RGBA()
	assert.Equal(t, uint32(255), r>>8) // each color component has been scaled by alpha (<<8)
	assert.Equal(t, uint32(0), g)
	assert.Equal(t, uint32(0), b)
}

func TestElementHandleWaitForSelector(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)
	p.SetContent(`<div class="root"></div>`, nil)

	root := p.Query(".root")
	p.Evaluate(tb.toGojaValue(`
        () => {
		setTimeout(() => {
			const div = document.createElement('div');
			div.className = 'element-to-appear';
			div.appendChild(document.createTextNode("Hello World"));
			root = document.querySelector('.root');
			root.appendChild(div);
			}, 100);
		}
	`))
	element := root.WaitForSelector(".element-to-appear", tb.toGojaValue(struct {
		Timeout int64 `js:"timeout"`
	}{Timeout: 1000}))

	require.NotNil(t, element, "expected element to have been found after wait")

	element.Dispose()
}
