package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/browser"
	"go.k6.io/k6/internal/js/modules/k6/browser/chromium"
	"go.k6.io/k6/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestBrowserTypeConnect(t *testing.T) {
	t.Parallel()

	// Start a test browser so we can get its WS URL
	// and use it to connect through BrowserType.Connect.
	tb := newTestBrowser(t)
	vu := k6test.NewVU(t)
	bt := chromium.NewBrowserType(vu)
	vu.ActivateVU()

	b, err := bt.Connect(context.Background(), context.Background(), tb.wsURL)
	require.NoError(t, err)
	_, err = b.NewPage(nil)
	require.NoError(t, err)
}

func TestBrowserTypeLaunchToConnect(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	bp := newTestBrowserProxy(t, tb)

	// Export WS URL env var
	// pointing to test browser proxy
	vu := k6test.NewVU(t, env.ConstLookup(env.WebSocketURLs, bp.wsURL()))

	// We have to call launch method through JS API in sobek
	// to take mapping layer into account, instead of calling
	// BrowserType.Launch method directly
	root := browser.New()
	mod := root.NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)

	vu.ActivateVU()
	vu.StartIteration(t)

	vu.SetVar(t, "browser", jsMod.Browser)
	_, err := vu.RunAsync(t, `
		const p = await browser.newPage();
		await p.close();
	`)
	require.NoError(t, err)

	// Verify the proxy, which's WS URL was set as
	// K6_BROWSER_WS_URL, has received a connection req
	require.True(t, bp.connected)
	// Verify that no new process pids have been added
	// to pid registry
	require.Len(t, root.PidRegistry.Pids(), 0)
}
