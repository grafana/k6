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
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common"
)

func TestLaunchOptionsSlowMo(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	tb := newTestBrowser(t, withFileServer())

	t.Run("Page", func(t *testing.T) {
		t.Run("check", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Check(".check", nil)
			})
		})
		t.Run("click", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Click("button", nil)
			})
		})
		t.Run("dblClick", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Dblclick("button", nil)
			})
		})
		t.Run("dispatchEvent", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.DispatchEvent("button", "click", goja.Null(), nil)
			})
		})
		t.Run("emulateMedia", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.EmulateMedia(tb.rt.ToValue(struct {
					Media string `js:"media"`
				}{
					Media: "print",
				}))
			})
		})
		t.Run("evaluate", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Evaluate(tb.rt.ToValue("() => void 0"))
			})
		})
		t.Run("evaluateHandle", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.EvaluateHandle(tb.rt.ToValue("() => window"))
			})
		})
		t.Run("fill", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Fill(".fill", "foo", nil)
			})
		})
		t.Run("focus", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Focus("button", nil)
			})
		})
		t.Run("goto", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Goto("about:blank", nil)
			})
		})
		t.Run("hover", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Hover("button", nil)
			})
		})
		t.Run("press", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Press("button", "Enter", nil)
			})
		})
		t.Run("reload", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Reload(nil)
			})
		})
		t.Run("setContent", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.SetContent("hello world", nil)
			})
		})
		/*t.Run("setInputFiles", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *Browser, p api.Page) {
				p.SetInputFiles(".file", nil, nil)
			})
		})*/
		t.Run("selectOption", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.SelectOption("select", tb.rt.ToValue("foo"), nil)
			})
		})
		t.Run("setViewportSize", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.SetViewportSize(nil)
			})
		})
		t.Run("type", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Type(".fill", "a", nil)
			})
		})
		t.Run("uncheck", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p api.Page) {
				p.Uncheck(".uncheck", nil)
			})
		})
	})

	t.Run("Frame", func(t *testing.T) {
		t.Run("check", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Check(".check", nil)
			})
		})
		t.Run("click", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Click("button", nil)
			})
		})
		t.Run("dblClick", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Dblclick("button", nil)
			})
		})
		t.Run("dispatchEvent", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.DispatchEvent("button", "click", goja.Null(), nil)
			})
		})
		t.Run("evaluate", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Evaluate(tb.rt.ToValue("() => void 0"))
			})
		})
		t.Run("evaluateHandle", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.EvaluateHandle(tb.rt.ToValue("() => window"))
			})
		})
		t.Run("fill", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Fill(".fill", "foo", nil)
			})
		})
		t.Run("focus", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Focus("button", nil)
			})
		})
		t.Run("goto", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Goto("about:blank", nil)
			})
		})
		t.Run("hover", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Hover("button", nil)
			})
		})
		t.Run("press", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Press("button", "Enter", nil)
			})
		})
		t.Run("setContent", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.SetContent("hello world", nil)
			})
		})
		/*t.Run("setInputFiles", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *Browser, f api.Frame) {
				f.SetInputFiles(".file", nil, nil)
			})
		})*/
		t.Run("selectOption", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.SelectOption("select", tb.rt.ToValue("foo"), nil)
			})
		})
		t.Run("type", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Type(".fill", "a", nil)
			})
		})
		t.Run("uncheck", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f api.Frame) {
				f.Uncheck(".uncheck", nil)
			})
		})
	})

	t.Run("ElementHandle", func(t *testing.T) {
	})
}

func testSlowMoImpl(t *testing.T, tb *testBrowser, fn func(*testBrowser)) {
	hooks := common.GetHooks(tb.ctx)
	currentHook := hooks.Get(common.HookApplySlowMo)
	chCalled := make(chan bool, 1)
	defer hooks.Register(common.HookApplySlowMo, currentHook)
	hooks.Register(common.HookApplySlowMo, func(ctx context.Context) {
		currentHook(ctx)
		chCalled <- true
	})

	didSlowMo := false
	go fn(tb)
	select {
	case <-tb.ctx.Done():
	case <-chCalled:
		didSlowMo = true
	}

	require.True(t, didSlowMo, "expected action to have been slowed down")
}

func testPageSlowMoImpl(t *testing.T, tb *testBrowser, fn func(*testBrowser, api.Page)) {
	p := tb.NewPage(nil)

	p.SetContent(`
		<button>a</button>
		<input type="checkbox" class="check">
		<input type="checkbox" checked=true class="uncheck">
		<input class="fill">
		<select>
		<option>foo</option>
		</select>
		<input type="file" class="file">
    	`, nil)
	testSlowMoImpl(t, tb, func(tb *testBrowser) { fn(tb, p) })
}

func testFrameSlowMoImpl(t *testing.T, tb *testBrowser, fn func(bt *testBrowser, f api.Frame)) {
	p := tb.NewPage(nil)

	f := tb.attachFrame(p, "frame1", tb.staticURL("empty.html"))
	f.SetContent(`
		<button>a</button>
		<input type="checkbox" class="check">
		<input type="checkbox" checked=true class="uncheck">
		<input class="fill">
		<select>
		  <option>foo</option>
		</select>
		<input type="file" class="file">
    	`, nil)
	testSlowMoImpl(t, tb, func(tb *testBrowser) { fn(tb, f) })
}
