/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package tests

import (
	_ "embed"
	"fmt"
	"reflect"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

//go:embed static/mouse_helper.js
var mouseHelperScriptSource string

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

func TestElementHandleClick(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.SetContent(htmlInputButton, nil)

	button := p.Query("button")
	button.Click(tb.rt.ToValue(struct {
		NoWaitAfter bool `js:"noWaitAfter"`
	}{
		NoWaitAfter: true, // FIX: this is just a workaround because navigation is never triggered and we'd be waiting for it to happen otherwise!
	}))

	result := p.Evaluate(tb.rt.ToValue("() => window['result']")).(goja.Value)
	switch result.ExportType().Kind() {
	case reflect.String:
		assert.Equal(t, result.String(), "Clicked", "expected button to be clicked, but got %q", result.String())
	default:
		t.Fail()
	}
}

func TestElementHandleClickWithNodeRemoved(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.SetContent(htmlInputButton, nil)

	// Remove all nodes
	p.Evaluate(tb.rt.ToValue("() => delete window['Node']"))

	button := p.Query("button")
	button.Click(tb.rt.ToValue(struct {
		NoWaitAfter bool `js:"noWaitAfter"`
	}{
		NoWaitAfter: true, // FIX: this is just a workaround because navigation is never triggered and we'd be waiting for it to happen otherwise!
	}))

	result := p.Evaluate(tb.rt.ToValue("() => window['result']")).(goja.Value)
	switch result.ExportType().Kind() {
	case reflect.String:
		assert.Equal(t, result.String(), "Clicked", "expected button to be clicked, but got %q", result.String())
	default:
		t.Fail()
	}
}

func TestElementHandleClickWithDetachedNode(t *testing.T) {
	tb := newTestBrowser(t)
	p := tb.NewPage(nil)

	p.SetContent(htmlInputButton, nil)

	button := p.Query("button")

	// Detach node
	p.Evaluate(tb.rt.ToValue("button => button.remove()"), tb.rt.ToValue(button))

	// We expect the click to fail with the correct error raised
	var errorMsg string
	panicTestFn := func() {
		defer func() {
			if err := recover(); err != nil {
				errorMsg = err.(*goja.Object).String()
			}
		}()
		button.Click(tb.rt.ToValue(struct {
			NoWaitAfter bool `js:"noWaitAfter"`
		}{
			// FIX: this is just a workaround because navigation is never triggered and we'd be waiting for
			// it to happen otherwise!
			NoWaitAfter: true,
		}))
	}
	panicTestFn()
	assert.Contains(t, errorMsg, "element is not attached to the DOM",
		"expected click to result in correct error to be thrown")
}
