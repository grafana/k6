package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

func TestMouseActions(t *testing.T) {
	t.Parallel()

	t.Run("click", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		m := p.GetMouse()

		// Set up a page with a button that changes text when clicked
		buttonHTML := `
			<button onclick="this.innerHTML='Clicked!'">Click me</button>
		`
		err := p.SetContent(buttonHTML, nil)
		require.NoError(t, err)
		button, err := p.Query("button")
		require.NoError(t, err)

		// Simulate a click at the button coordinates
		box := button.BoundingBox()
		require.NoError(t, m.Click(box.X, box.Y, common.NewMouseClickOptions()))

		// Verify the button's text changed
		text, ok, err := button.TextContent()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "Clicked!", text)
	})

	t.Run("double_click", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		m := p.GetMouse()

		// Set up a page with a button that changes text on double click and also counts clicks
		buttonHTML := `
			<script>window.clickCount = 0;</script>
			<button
				onclick="document.getElementById('clicks').innerHTML = ++window.clickCount;"
				ondblclick="this.innerHTML='Double Clicked!';">Click me</button>
			<div id="clicks"></div>
		`
		err := p.SetContent(buttonHTML, nil)
		require.NoError(t, err)
		button, err := p.Query("button")
		require.NoError(t, err)

		// Get the button's bounding box for accurate clicking
		box := button.BoundingBox()

		// Simulate a double click at the button coordinates
		require.NoError(t, m.DblClick(box.X, box.Y, common.NewMouseDblClickOptions()))

		// Verify the button's text changed
		text, ok, err := button.TextContent()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "Double Clicked!", text)

		// Also verify that the element was clicked twice
		clickCountDiv, err := p.Query("div#clicks")
		require.NoError(t, err)
		text, ok, err = clickCountDiv.TextContent()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "2", text)
	})

	t.Run("move", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		m := p.GetMouse()

		// Set up a page with an area that detects mouse move
		areaHTML := `
			<div
				onmousemove="this.innerHTML='Mouse Moved';"
				style="width:100px;height:100px;"
			></div>
		`
		err := p.SetContent(areaHTML, nil)
		require.NoError(t, err)
		area, err := p.Query("div")
		require.NoError(t, err)

		// Simulate mouse move within the div
		box := area.BoundingBox()
		require.NoError(t, m.Move(box.X+50, box.Y+50, common.NewMouseMoveOptions())) // Move to the center of the div
		text, ok, err := area.TextContent()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "Mouse Moved", text)
	})

	t.Run("move_down_up", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		p := tb.NewPage(nil)
		m := p.GetMouse()

		// Set up a page with a button that tracks mouse down and up
		buttonHTML := `
			<button
				onmousedown="this.innerHTML='Mouse Down';"
				onmouseup="this.innerHTML='Mouse Up';"
			>Mouse</button>
		`
		err := p.SetContent(buttonHTML, nil)
		require.NoError(t, err)
		button, err := p.Query("button")
		require.NoError(t, err)

		box := button.BoundingBox()
		require.NoError(t, m.Move(box.X, box.Y, common.NewMouseMoveOptions()))
		require.NoError(t, m.Down(common.NewMouseDownUpOptions()))
		text, ok, err := button.TextContent()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "Mouse Down", text)
		require.NoError(t, m.Up(common.NewMouseDownUpOptions()))
		text, ok, err = button.TextContent()
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "Mouse Up", text)
	})
}
