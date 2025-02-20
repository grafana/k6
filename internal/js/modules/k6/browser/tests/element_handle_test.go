package tests

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image/png"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
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
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	err := p.SetContent(`<div style="display:none">hello</div>`, nil)
	require.NoError(t, err)
	element, err := p.Query("div")
	require.NoError(t, err)
	require.Nil(t, element.BoundingBox())
}

func TestElementHandleBoundingBoxSVG(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	err := p.SetContent(`
		<svg xmlns="http://www.w3.org/2000/svg" width="500" height="500">
			<rect id="theRect" x="30" y="50" width="200" height="300"></rect>
		</svg>
	`, nil)
	require.NoError(t, err)

	element, err := p.Query("#therect")
	require.NoError(t, err)

	bbox := element.BoundingBox()
	pageFn := `e => {
        const rect = e.getBoundingClientRect();
        return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
    }`
	box, err := p.Evaluate(pageFn, element)
	require.NoError(t, err)
	rect := convert(t, box, &common.Rect{})
	require.EqualValues(t, bbox, rect)
}

func TestElementHandleClick(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	err := p.SetContent(htmlInputButton, nil)
	require.NoError(t, err)

	button, err := p.Query("button")
	require.NoError(t, err)

	opts := common.NewElementHandleClickOptions(button.Timeout())
	// FIX: this is just a workaround because navigation is never triggered
	// and we'd be waiting for it to happen otherwise!
	opts.NoWaitAfter = true
	err = button.Click(opts)
	require.NoError(t, err)

	res, err := p.Evaluate(`() => window['result']`)
	require.NoError(t, err)
	assert.Equal(t, res, "Clicked")
}

func TestElementHandleClickWithNodeRemoved(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	err := p.SetContent(htmlInputButton, nil)
	require.NoError(t, err)

	// Remove all nodes
	_, err = p.Evaluate(`() => delete window['Node']`)
	require.NoError(t, err)

	button, err := p.Query("button")
	require.NoError(t, err)

	opts := common.NewElementHandleClickOptions(button.Timeout())
	// FIX: this is just a workaround because navigation is never triggered
	// and we'd be waiting for it to happen otherwise!
	opts.NoWaitAfter = true
	err = button.Click(opts)
	require.NoError(t, err)

	res, err := p.Evaluate(`() => window['result']`)
	require.NoError(t, err)
	assert.Equal(t, res, "Clicked")
}

func TestElementHandleClickWithDetachedNode(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	err := p.SetContent(htmlInputButton, nil)
	require.NoError(t, err)
	button, err := p.Query("button")
	require.NoError(t, err)

	// Detach node to panic when clicked
	_, err = p.Evaluate(`button => button.remove()`, button)
	require.NoError(t, err)

	opts := common.NewElementHandleClickOptions(button.Timeout())
	// FIX: this is just a workaround because navigation is never triggered
	// and we'd be waiting for it to happen otherwise!
	opts.NoWaitAfter = true
	err = button.Click(opts)
	assert.ErrorContains(
		t, err,
		"element is not attached to the DOM",
		"expected click to result in correct error to panic",
	)
}

func TestElementHandleClickConcealedLink(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip() // wrong result
	}

	const (
		wantBefore = "🙈"
		wantAfter  = "🐵"
	)

	tb := newTestBrowser(t, withFileServer())

	bcopts := common.DefaultBrowserContextOptions()
	bcopts.Viewport = common.Viewport{
		Width:  500,
		Height: 240,
	}
	bc, err := tb.NewContext(bcopts)
	require.NoError(t, err)

	p, err := bc.NewPage()
	require.NoError(t, err)

	clickResult := func() (any, error) {
		const cmd = `
			() => window.clickResult
		`
		return p.Evaluate(cmd)
	}
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	resp, err := p.Goto(
		tb.staticURL("/concealed_link.html"),
		opts,
	)
	require.NotNil(t, resp)
	require.NoError(t, err)
	result, err := clickResult()
	require.NoError(t, err)
	require.Equal(t, wantBefore, result)

	err = p.Click("#concealed", common.NewFrameClickOptions(p.Timeout()))
	require.NoError(t, err)
	result, err = clickResult()
	require.NoError(t, err)
	require.Equal(t, wantAfter, result)
}

func TestElementHandleNonClickable(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withFileServer())

	bctx, err := tb.NewContext(nil)
	require.NoError(t, err)
	p, err := bctx.NewPage()
	require.NoError(t, err)

	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	resp, err := p.Goto(
		tb.staticURL("/non_clickable.html"),
		opts,
	)
	require.NotNil(t, resp)
	require.NoError(t, err)

	err = p.Click("#non-clickable", common.NewFrameClickOptions(p.Timeout()))
	require.Errorf(t, err, "element should not be clickable")
}

func TestElementHandleGetAttribute(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el" href="null">Something</a>`, nil)
	require.NoError(t, err)

	el, err := p.Query("#el")
	require.NoError(t, err)

	got, ok, err := el.GetAttribute("href")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "null", got)
}

func TestElementHandleGetAttributeMissing(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el">Something</a>`, nil)
	require.NoError(t, err)

	el, err := p.Query("#el")
	require.NoError(t, err)

	got, ok, err := el.GetAttribute("missing")
	require.NoError(t, err)
	require.False(t, ok)
	assert.Equal(t, "", got)
}

func TestElementHandleGetAttributeEmpty(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<a id="el" empty>Something</a>`, nil)
	require.NoError(t, err)

	el, err := p.Query("#el")
	require.NoError(t, err)

	got, ok, err := el.GetAttribute("empty")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "", got)
}

func TestElementHandleInputValue(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	err := p.SetContent(`
		<input value="hello1">
		<select><option value="hello2" selected></option></select>
		<textarea>hello3</textarea>
    `, nil)
	require.NoError(t, err)

	element, err := p.Query("input")
	require.NoError(t, err)

	value, err := element.InputValue(common.NewElementHandleBaseOptions(element.Timeout()))
	require.NoError(t, err)
	require.NoError(t, element.Dispose())
	assert.Equal(t, value, "hello1", `expected input value "hello1", got %q`, value)

	element, err = p.Query("select")
	require.NoError(t, err)

	value, err = element.InputValue(common.NewElementHandleBaseOptions(element.Timeout()))
	require.NoError(t, err)
	require.NoError(t, element.Dispose())
	assert.Equal(t, value, "hello2", `expected input value "hello2", got %q`, value)

	element, err = p.Query("textarea")
	require.NoError(t, err)

	value, err = element.InputValue(common.NewElementHandleBaseOptions(element.Timeout()))
	require.NoError(t, err)
	require.NoError(t, element.Dispose())
	assert.Equal(t, value, "hello3", `expected input value "hello3", got %q`, value)
}

func TestElementHandleIsChecked(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	err := p.SetContent(`<input type="checkbox" checked>`, nil)
	require.NoError(t, err)
	element, err := p.Query("input")
	require.NoError(t, err)

	checked, err := element.IsChecked()
	require.NoError(t, err)
	assert.True(t, checked, "expected checkbox to be checked")
	require.NoError(t, element.Dispose())

	err = p.SetContent(`<input type="checkbox">`, nil)
	require.NoError(t, err)
	element, err = p.Query("input")
	require.NoError(t, err)
	checked, err = element.IsChecked()
	require.NoError(t, err)
	assert.False(t, checked, "expected checkbox to be unchecked")
	require.NoError(t, element.Dispose())
}

func TestElementHandleQueryAll(t *testing.T) {
	t.Parallel()

	const (
		wantLiLen = 2
		query     = "li.ali"
	)

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`
		<ul id="aul">
			<li class="ali">1</li>
			<li class="ali">2</li>
		</ul>
  	`, nil)
	require.NoError(t, err)

	t.Run("element_handle", func(t *testing.T) {
		t.Parallel()

		el, err := p.Query("#aul")
		require.NoError(t, err)

		els, err := el.QueryAll(query)
		require.NoError(t, err)

		assert.Equal(t, wantLiLen, len(els))
	})
	t.Run("page", func(t *testing.T) {
		t.Parallel()

		els, err := p.QueryAll(query)
		require.NoError(t, err)

		assert.Equal(t, wantLiLen, len(els))
	})
	t.Run("frame", func(t *testing.T) {
		t.Parallel()

		els, err := p.MainFrame().QueryAll(query)
		require.NoError(t, err)
		assert.Equal(t, wantLiLen, len(els))
	})
}

type mockPersister struct{}

func (m *mockPersister) Persist(_ context.Context, _ string, _ io.Reader) (err error) {
	return nil
}

func TestElementHandleScreenshot(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	viewportSize := tb.toSobekValue(struct {
		Width  float64 `js:"width"`
		Height float64 `js:"height"`
	}{Width: 800, Height: 600})
	s := new(common.Size)
	require.NoError(t, s.Parse(tb.context(), viewportSize))

	err := p.SetViewportSize(s)
	require.NoError(t, err)

	_, err = p.Evaluate(`
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
	`)
	require.NoError(t, err)

	elem, err := p.Query("div")
	require.NoError(t, err)

	buf, err := elem.Screenshot(
		common.NewElementHandleScreenshotOptions(elem.Timeout()),
		&mockPersister{},
	)
	require.NoError(t, err)

	reader := bytes.NewReader(buf)
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
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)
	err := p.SetContent(`<div class="root"></div>`, nil)
	require.NoError(t, err)

	root, err := p.Query(".root")
	require.NoError(t, err)

	_, err = p.Evaluate(`
        () => {
			setTimeout(() => {
				const div = document.createElement('div');
				div.className = 'element-to-appear';
				div.appendChild(document.createTextNode("Hello World"));
				root = document.querySelector('.root');
				root.appendChild(div);
			}, 100);
		}
	`)
	require.NoError(t, err)

	element, err := root.WaitForSelector(".element-to-appear", common.NewFrameWaitForSelectorOptions(time.Second))
	require.NoError(t, err)
	require.NotNil(t, element, "expected element to have been found after wait")

	require.NoError(t, element.Dispose())
}

func TestElementHandlePress(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)

	p := tb.NewPage(nil)

	err := p.SetContent(`<input>`, nil)
	require.NoError(t, err)

	el, err := p.Query("input")
	require.NoError(t, err)

	require.NoError(t, el.Press("Shift+KeyA", common.NewElementHandlePressOptions(el.Timeout())))
	require.NoError(t, el.Press("KeyB", common.NewElementHandlePressOptions(el.Timeout())))
	require.NoError(t, el.Press("Shift+KeyC", common.NewElementHandlePressOptions(el.Timeout())))

	v, err := el.InputValue(common.NewElementHandleBaseOptions(el.Timeout()))
	require.NoError(t, err)
	require.Equal(t, "AbC", v)
}

func TestElementHandleQuery(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<div id="foo">hello</div>`, nil)
	require.NoError(t, err)

	element, err := p.Query("bar")

	require.NoError(t, err)
	require.Nil(t, element)
}

func TestElementHandleTextContent(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<div id="el">Something</div>`, nil)
	require.NoError(t, err)

	el, err := p.Query("#el")
	require.NoError(t, err)

	got, ok, err := el.TextContent()
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "Something", got)
}

func TestElementHandleTextContentMissing(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	// document never has text content.
	js, err := p.EvaluateHandle(`() => document`)
	require.NoError(t, err)
	_, ok, err := js.AsElement().TextContent()
	require.NoError(t, err)
	require.False(t, ok)
}

func TestElementHandleTextContentEmpty(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)
	err := p.SetContent(`<div id="el"/>`, nil)
	require.NoError(t, err)

	el, err := p.Query("#el")
	require.NoError(t, err)

	got, ok, err := el.TextContent()
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, got)
}

func TestElementHandleSetChecked(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	err := p.SetContent(`<input type="checkbox">`, nil)
	require.NoError(t, err)
	element, err := p.Query("input")
	require.NoError(t, err)
	checked, err := element.IsChecked()
	require.NoError(t, err)
	require.False(t, checked, "expected checkbox to be unchecked")

	err = element.SetChecked(true, common.NewElementHandleSetCheckedOptions(element.Timeout()))
	require.NoError(t, err)
	checked, err = element.IsChecked()
	require.NoError(t, err)
	assert.True(t, checked, "expected checkbox to be checked")

	err = element.SetChecked(false, common.NewElementHandleSetCheckedOptions(element.Timeout()))
	require.NoError(t, err)
	checked, err = element.IsChecked()
	require.NoError(t, err)
	assert.False(t, checked, "expected checkbox to be unchecked")
}
