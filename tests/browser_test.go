package tests

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/browser"
	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/env"
	"github.com/grafana/xk6-browser/k6ext/k6test"
)

func TestBrowserNewPage(t *testing.T) {
	b := newTestBrowser(t)
	p1 := b.NewPage(nil)
	c := b.Context()
	assert.NotNil(t, c)

	_, err := b.Browser.NewPage(nil)
	assert.EqualError(t, err, "new page: existing browser context must be closed before creating a new one")

	err = p1.Close(nil)
	require.NoError(t, err)
	c = b.Context()
	assert.NotNil(t, c)

	_, err = b.Browser.NewPage(nil)
	assert.EqualError(t, err, "new page: existing browser context must be closed before creating a new one")

	b.Context().Close()
	c = b.Context()
	assert.Nil(t, c)

	_ = b.NewPage(nil)
	c = b.Context()
	assert.NotNil(t, c)
}

func TestBrowserNewContext(t *testing.T) {
	b := newTestBrowser(t)
	bc1, err := b.NewContext(nil)
	assert.NoError(t, err)
	c := b.Context()
	assert.NotNil(t, c)

	_, err = b.NewContext(nil)
	assert.EqualError(t, err, "existing browser context must be closed before creating a new one")

	bc1.Close()
	c = b.Context()
	assert.Nil(t, c)

	_, err = b.NewContext(nil)
	assert.NoError(t, err)
	c = b.Context()
	assert.NotNil(t, c)
}

func TestTmpDirCleanup(t *testing.T) {
	t.Parallel()

	const tmpDirPath = "./1/"
	err := os.Mkdir(tmpDirPath, os.ModePerm)
	require.NoError(t, err)

	defer func() {
		err = os.Remove(tmpDirPath)
		require.NoError(t, err)
	}()

	b := newTestBrowser(
		t,
		withSkipClose(),
		withEnvLookup(env.ConstLookup("TMPDIR", tmpDirPath)),
	)
	p := b.NewPage(nil)
	err = p.Close(nil)
	require.NoError(t, err)

	matches, err := filepath.Glob(tmpDirPath + "xk6-browser-data-*")
	assert.NoError(t, err)
	assert.NotEmpty(t, matches, "a dir should exist that matches the pattern `xk6-browser-data-*`")

	b.Close()

	matches, err = filepath.Glob(tmpDirPath + "xk6-browser-data-*")
	assert.NoError(t, err)
	assert.Empty(t, matches, "a dir shouldn't exist which matches the pattern `xk6-browser-data-*`")
}

func TestBrowserOn(t *testing.T) {
	t.Parallel()

	script := `
	const result = b.on('%s')
	log(result);`

	t.Run("err_wrong_event", func(t *testing.T) {
		t.Parallel()

		b := newTestBrowser(t)
		require.NoError(t, b.runtime().Set("b", b.Browser))

		_, err := b.runJavaScript(script, "wrongevent")
		require.Error(t, err)
		assert.ErrorContains(t, err, `unknown browser event: "wrongevent", must be "disconnected"`)
	})

	t.Run("ok_promise_resolved", func(t *testing.T) {
		t.Parallel()

		var (
			b   = newTestBrowser(t, withSkipClose())
			rt  = b.runtime()
			log []string
		)

		require.NoError(t, rt.Set("b", b.Browser))
		require.NoError(t, rt.Set("log", func(s string) { log = append(log, s) }))

		time.AfterFunc(100*time.Millisecond, b.Browser.Close)
		_, err := b.runJavaScript(script, "disconnected")
		require.NoError(t, err)
		assert.Contains(t, log, "true")
	})

	t.Run("ok_promise_rejected", func(t *testing.T) {
		t.Parallel()

		var (
			b   = newTestBrowser(t)
			rt  = b.runtime()
			log []string
		)

		require.NoError(t, rt.Set("b", b.Browser))
		require.NoError(t, rt.Set("log", func(s string) { log = append(log, s) }))

		time.AfterFunc(100*time.Millisecond, b.cancelContext)
		_, err := b.runJavaScript(script, "disconnected")
		assert.ErrorContains(t, err, "browser.on promise rejected: context canceled")
	})
}

// This only works for Chrome!
func TestBrowserVersion(t *testing.T) {
	const re = `^\d+\.\d+\.\d+\.\d+$`
	r, _ := regexp.Compile(re)
	ver := newTestBrowser(t).Version()
	assert.Regexp(t, r, ver, "expected browser version to match regex %q, but found %q", re, ver)
}

// This only works for Chrome!
// TODO: Improve this test, see:
// https://github.com/grafana/xk6-browser/pull/51#discussion_r742696736
func TestBrowserUserAgent(t *testing.T) {
	b := newTestBrowser(t)

	// testBrowserVersion() tests the version already
	// just look for "Headless" in UserAgent
	ua := b.UserAgent()
	if prefix := "Mozilla/5.0"; !strings.HasPrefix(ua, prefix) {
		t.Errorf("UserAgent should start with %q, but got: %q", prefix, ua)
	}
	assert.Contains(t, ua, "Headless")
}

func TestBrowserCrashErr(t *testing.T) {
	// create a new VU in an environment that requires a bad remote-debugging-port.
	vu := k6test.NewVU(t, env.ConstLookup(env.BrowserArguments, "remote-debugging-port=99999"))

	mod := browser.New().NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)

	vu.ActivateVU()
	vu.StartIteration(t)

	rt := vu.Runtime()
	require.NoError(t, rt.Set("browser", jsMod.Browser))
	_, err := rt.RunString(`
		const p = browser.newPage();
		p.close();
	`)
	assert.ErrorContains(t, err, "launching browser: Invalid devtools server port")
}

func TestBrowserLogIterationID(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withLogCache())

	var (
		iterID     = common.GetIterationID(tb.ctx)
		tracedEvts int
	)

	require.NotEmpty(t, iterID)

	tb.logCache.mu.RLock()
	defer tb.logCache.mu.RUnlock()

	require.NotEmpty(t, tb.logCache.entries)

	for _, evt := range tb.logCache.entries {
		for k, v := range evt.Data {
			if k == "iteration_id" {
				assert.Equal(t, iterID, v)
				tracedEvts++
			}
		}
	}

	assert.Equal(t, len(tb.logCache.entries), tracedEvts)
}

func TestMultiBrowserPanic(t *testing.T) {
	var b1, b2 *testBrowser

	// run it in a test to kick in the Cleanup() in testBrowser.
	t.Run("browsers", func(t *testing.T) {
		b1 = newTestBrowser(t)
		b2 = newTestBrowser(t)

		bctx, err := b1.NewContext(nil)
		require.NoError(t, err)
		p1, err := bctx.NewPage()
		require.NoError(t, err, "failed to create page #1")

		func() {
			defer func() { _ = recover() }()
			p1.GoBack(nil)
		}()
	})

	// FindProcess only returns alive/dead processes on nixes.
	// Sending Interrupt on Windows is not implemented.
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	p1, err := os.FindProcess(b1.pid)
	require.NoError(t, err, "failed to find process #1")
	p2, err := os.FindProcess(b2.pid)
	require.NoError(t, err, "failed to find process #2")
	err = p1.Signal(syscall.Signal(0))
	assert.Error(t, err, "process #1 should be dead, but exists")
	err = p2.Signal(syscall.Signal(0))
	assert.Error(t, err, "process #2 should be dead, but exists")
}

func TestBrowserMultiClose(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withSkipClose(), withLogCache())

	require.NotPanicsf(t, tb.Close, "first call to browser.close should not panic")
	require.NotPanicsf(t, tb.Close, "second call to browser.close should not panic")
	tb.logCache.assertContains(t, "browser.close only once")
}

func TestMultiConnectToSingleBrowser(t *testing.T) {
	tb := newTestBrowser(t, withSkipClose())
	defer tb.Close()

	ctx := context.Background()

	b1, err := tb.browserType.Connect(ctx, tb.wsURL)
	require.NoError(t, err)
	bctx1, err := b1.NewContext(nil)
	require.NoError(t, err)
	p1, err := bctx1.NewPage()
	require.NoError(t, err, "failed to create page #1")

	b2, err := tb.browserType.Connect(ctx, tb.wsURL)
	require.NoError(t, err)
	bctx2, err := b2.NewContext(nil)
	require.NoError(t, err)

	err = p1.Close(nil)
	require.NoError(t, err, "failed to close page #1")
	bctx1.Close()

	p2, err := bctx2.NewPage()
	require.NoError(t, err, "failed to create page #2")
	err = p2.Close(nil)
	require.NoError(t, err, "failed to close page #2")
}
