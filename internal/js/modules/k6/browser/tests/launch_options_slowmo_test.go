package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

func TestBrowserOptionsSlowMo(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip()
	}

	t.Run("Page", func(t *testing.T) {
		t.Parallel()
		t.Run("check", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				assert.NoError(t, p.Check(".check", common.NewFrameCheckOptions(p.MainFrame().Timeout())))
			})
		})
		t.Run("click", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				err := p.Click("button", common.NewFrameClickOptions(p.Timeout()))
				assert.NoError(t, err)
			})
		})
		t.Run("dblClick", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				err := p.Dblclick("button", common.NewFrameDblClickOptions(p.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("dispatchEvent", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				err := p.DispatchEvent("button", "click", nil, common.NewFrameDispatchEventOptions(p.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("emulateMedia", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				popts := common.NewPageEmulateMediaOptions(p)
				require.NoError(t, popts.Parse(tb.context(), tb.toSobekValue(struct {
					Media string `js:"media"`
				}{
					Media: "print",
				})))
				err := p.EmulateMedia(popts)
				require.NoError(t, err)
			})
		})
		t.Run("evaluate", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				_, err := p.Evaluate(`() => void 0`)
				require.NoError(t, err)
			})
		})
		t.Run("evaluateHandle", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				_, err := p.EvaluateHandle(`() => window`)
				require.NoError(t, err)
			})
		})
		t.Run("fill", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				err := p.Fill(".fill", "foo", common.NewFrameFillOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("focus", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				err := p.Focus("button", common.NewFrameBaseOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("goto", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				opts := &common.FrameGotoOptions{
					Timeout: common.DefaultTimeout,
				}
				_, err := p.Goto(
					common.BlankPage,
					opts,
				)
				require.NoError(t, err)
			})
		})
		t.Run("hover", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				err := p.Hover("button", common.NewFrameHoverOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("press", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				err := p.Press("button", "Enter", common.NewFramePressOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("reload", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				opts := common.NewPageReloadOptions(common.LifecycleEventLoad, p.MainFrame().NavigationTimeout())
				_, err := p.Reload(opts)
				require.NoError(t, err)
			})
		})
		t.Run("setContent", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				err := p.SetContent("hello world", nil)
				require.NoError(t, err)
			})
		})
		/*t.Run("setInputFiles", func(t *testing.T) {
			testPageSlowMoImpl(t, tb, func(_ *Browser, p *common.Page) {
				p.SetInputFiles(".file", nil, nil)
			})
		})*/
		t.Run("selectOption", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				_, err := p.SelectOption("select", []any{"foo"}, common.NewFrameSelectOptionOptions(p.MainFrame().Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("type", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				err := p.Type(".fill", "a", common.NewFrameTypeOptions(p.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("uncheck", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testPageSlowMoImpl(t, tb, func(_ *testBrowser, p *common.Page) {
				assert.NoError(t, p.Uncheck(".uncheck", common.NewFrameUncheckOptions(p.Timeout())))
			})
		})
	})

	t.Run("Frame", func(t *testing.T) {
		t.Parallel()
		t.Run("check", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				assert.NoError(t, f.Check(".check", common.NewFrameCheckOptions(f.Timeout())))
			})
		})
		t.Run("click", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				err := f.Click("button", common.NewFrameClickOptions(f.Timeout()))
				assert.NoError(t, err)
			})
		})
		t.Run("dblClick", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				err := f.Dblclick("button", common.NewFrameDblClickOptions(f.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("dispatchEvent", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				err := f.DispatchEvent("button", "click", nil, common.NewFrameDispatchEventOptions(f.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("evaluate", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				_, err := f.Evaluate(`() => void 0`)
				require.NoError(t, err)
			})
		})
		t.Run("evaluateHandle", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				_, err := f.EvaluateHandle(`() => window`)
				require.NoError(t, err)
			})
		})
		t.Run("fill", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				err := f.Fill(".fill", "foo", common.NewFrameFillOptions(f.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("focus", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				err := f.Focus("button", common.NewFrameBaseOptions(f.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("goto", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				opts := &common.FrameGotoOptions{
					Timeout: common.DefaultTimeout,
				}
				_, _ = f.Goto(
					common.BlankPage,
					opts,
				)
			})
		})
		t.Run("hover", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				err := f.Hover("button", common.NewFrameHoverOptions(f.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("press", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				err := f.Press("button", "Enter", common.NewFramePressOptions(f.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("setContent", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				err := f.SetContent("hello world", nil)
				require.NoError(t, err)
			})
		})
		/*t.Run("setInputFiles", func(t *testing.T) {
			testFrameSlowMoImpl(t, tb, func(_ *Browser, f common.Frame) {
				f.SetInputFiles(".file", nil, nil)
			})
		})*/
		t.Run("selectOption", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				_, err := f.SelectOption("select", []any{"foo"}, common.NewFrameSelectOptionOptions(f.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("type", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				err := f.Type(".fill", "a", common.NewFrameTypeOptions(f.Timeout()))
				require.NoError(t, err)
			})
		})
		t.Run("uncheck", func(t *testing.T) {
			t.Parallel()
			tb := newTestBrowser(t, withFileServer())
			testFrameSlowMoImpl(t, tb, func(_ *testBrowser, f *common.Frame) {
				assert.NoError(t, f.Uncheck(".uncheck", common.NewFrameUncheckOptions(f.Timeout())))
			})
		})
	})
}

func testSlowMoImpl(t *testing.T, tb *testBrowser, fn func(*testBrowser)) {
	t.Helper()

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

func testPageSlowMoImpl(t *testing.T, tb *testBrowser, fn func(*testBrowser, *common.Page)) {
	t.Helper()

	p := tb.NewPage(nil)
	err := p.SetContent(`
		<button>a</button>
		<input type="checkbox" class="check">
		<input type="checkbox" checked=true class="uncheck">
		<input class="fill">
		<select>
		<option>foo</option>
		</select>
		<input type="file" class="file">
    	`, nil,
	)
	require.NoError(t, err)
	testSlowMoImpl(t, tb, func(tb *testBrowser) { fn(tb, p) })
}

func testFrameSlowMoImpl(t *testing.T, tb *testBrowser, fn func(bt *testBrowser, f *common.Frame)) {
	t.Helper()

	p := tb.NewPage(nil)

	pageFn := `
	async (frameId, url) => {
		const frame = document.createElement('iframe');
		frame.src = url;
		frame.id = frameId;
		document.body.appendChild(frame);
		await new Promise(x => frame.onload = x);
		return frame;
	}
	`

	h, err := p.EvaluateHandle(
		pageFn,
		"frame1",
		tb.staticURL("empty.html"),
	)
	require.NoError(tb.t, err)

	f, err := h.AsElement().ContentFrame()
	require.NoError(tb.t, err)

	err = f.SetContent(`
		<button>a</button>
		<input type="checkbox" class="check">
		<input type="checkbox" checked=true class="uncheck">
		<input class="fill">
		<select>
		  <option>foo</option>
		</select>
		<input type="file" class="file">
    	`, nil)
	require.NoError(tb.t, err)
	testSlowMoImpl(t, tb, func(tb *testBrowser) { fn(tb, f) })
}
