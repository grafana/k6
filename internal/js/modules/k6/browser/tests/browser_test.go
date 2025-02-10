package tests

import (
	"context"
	"errors"
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

	"go.k6.io/k6/internal/js/modules/k6/browser/browser"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/internal/js/modules/k6/browser/storage"
)

func TestBrowserNewPage(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t)
	p1 := b.NewPage(nil)
	c := b.Context()
	assert.NotNil(t, c)

	_, err := b.Browser.NewPage(nil)
	assert.EqualError(t, err, "new page: existing browser context must be closed before creating a new one")

	err = p1.Close()
	require.NoError(t, err)
	c = b.Context()
	assert.NotNil(t, c)

	_, err = b.Browser.NewPage(nil)
	assert.EqualError(t, err, "new page: existing browser context must be closed before creating a new one")

	require.NoError(t, b.Context().Close())
	c = b.Context()
	assert.Nil(t, c)

	_ = b.NewPage(nil)
	c = b.Context()
	assert.NotNil(t, c)
}

func TestBrowserNewContext(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t)
	bc1, err := b.NewContext(nil)
	assert.NoError(t, err)
	c := b.Context()
	assert.NotNil(t, c)

	_, err = b.NewContext(nil)
	assert.EqualError(t, err, "existing browser context must be closed before creating a new one")

	require.NoError(t, bc1.Close())
	c = b.Context()
	assert.Nil(t, c)

	_, err = b.NewContext(nil)
	assert.NoError(t, err)
	c = b.Context()
	assert.NotNil(t, c)
}

func TestTmpDirCleanup(t *testing.T) {
	t.Parallel()

	tmpDirPath, err := os.MkdirTemp("./", "") //nolint:forbidigo
	t.Cleanup(
		func() {
			err := os.RemoveAll(tmpDirPath) //nolint:forbidigo
			require.NoError(t, err)
		},
	)
	require.NoError(t, err)

	b := newTestBrowser(
		t,
		withSkipClose(),
		withEnvLookup(env.ConstLookup("TMPDIR", tmpDirPath)),
	)
	p := b.NewPage(nil)
	err = p.Close()
	require.NoError(t, err)

	matches, err := filepath.Glob(filepath.Join(tmpDirPath, storage.K6BrowserDataDirPattern))
	assert.NoError(t, err)
	assert.NotEmptyf(t, matches, "a dir should exist that matches the pattern %q", storage.K6BrowserDataDirPattern)

	b.Close()

	// We need to wait for something (k6 browser, chromium or the os) to
	// actually complete the removal of the directory. It's a race condition.
	// To try to mitigate the issue, we're adding a retry which waits half a
	// second if the dir still exits.
	for i := 0; i < 5; i++ {
		matches, err = filepath.Glob(filepath.Join(tmpDirPath, storage.K6BrowserDataDirPattern))
		assert.NoError(t, err)
		if len(matches) == 0 {
			break
		}
		time.Sleep(time.Millisecond * 500)
	}

	assert.Empty(t, matches, "a dir shouldn't exist which matches the pattern %q", storage.K6BrowserDataDirPattern)
}

func TestTmpDirCleanupOnContextClose(t *testing.T) {
	t.Parallel()

	tmpDirPath, err := os.MkdirTemp("./", "") //nolint:forbidigo
	t.Cleanup(
		func() {
			err := os.RemoveAll(tmpDirPath) //nolint:forbidigo
			require.NoError(t, err)
		},
	)
	require.NoError(t, err)

	b := newTestBrowser(
		t,
		withSkipClose(),
		withEnvLookup(env.ConstLookup("TMPDIR", tmpDirPath)),
	)

	matches, err := filepath.Glob(filepath.Join(tmpDirPath, storage.K6BrowserDataDirPattern))
	assert.NoError(t, err)
	assert.NotEmpty(t, matches, "a dir should exist that matches the pattern %q", storage.K6BrowserDataDirPattern)

	b.cancelContext()
	<-b.ctx.Done()

	require.NotPanicsf(t, b.Close, "first call to browser.close should not panic")

	matches, err = filepath.Glob(filepath.Join(tmpDirPath, storage.K6BrowserDataDirPattern))
	assert.NoError(t, err)
	assert.Empty(t, matches, "a dir shouldn't exist which matches the pattern %q", storage.K6BrowserDataDirPattern)
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
	t.Parallel()

	const re = `^\d+\.\d+\.\d+\.\d+$`
	r, err := regexp.Compile(re) //nolint:gocritic
	require.NoError(t, err)
	ver := newTestBrowser(t).Version()
	assert.Regexp(t, r, ver, "expected browser version to match regex %q, but found %q", re, ver)
}

// This only works for Chrome!
// TODO: Improve this test, see:
// https://go.k6.io/k6/js/modules/k6/browser/pull/51#discussion_r742696736
func TestBrowserUserAgent(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t)

	ua := b.UserAgent()
	if prefix := "Mozilla/5.0"; !strings.HasPrefix(ua, prefix) {
		t.Errorf("UserAgent should start with %q, but got: %q", prefix, ua)
	}
	// We default to removing the "Headless" part of the user agent string.
	assert.NotContains(t, ua, "Headless")
}

func TestBrowserCrashErr(t *testing.T) {
	// Skip until we get answer from Chromium team in an open issue
	// https://issues.chromium.org/issues/364089353.
	t.Skip("Skipping until we get response from Chromium team")

	t.Parallel()

	// create a new VU in an environment that requires a bad remote-debugging-port.
	vu := k6test.NewVU(t, env.ConstLookup(env.BrowserArguments, "remote-debugging-port=99999"))

	mod := browser.New().NewModuleInstance(vu)
	jsMod, ok := mod.Exports().Default.(*browser.JSModule)
	require.Truef(t, ok, "unexpected default mod export type %T", mod.Exports().Default)

	vu.ActivateVU()
	vu.StartIteration(t)

	vu.SetVar(t, "browser", jsMod.Browser)
	_, err := vu.RunAsync(t, `
		const p = await browser.newPage();
		await p.close();
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

//nolint:paralleltest
func TestMultiBrowserPanic(t *testing.T) {
	// this test should run sequentially.
	// don't use t.Parallel() here.

	var b1, b2 *testBrowser

	// run it in a test to kick in the Cleanup() in testBrowser.
	t.Run("browsers", func(t *testing.T) {
		b1 = newTestBrowser(t)
		b2 = newTestBrowser(t)

		func() {
			defer func() { _ = recover() }()
			k6ext.Panic(b1.ctx, "forcing a panic")
		}()
	})

	// FindProcess only returns alive/dead processes on nixes.
	// Sending Interrupt on Windows is not implemented.
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	assertProcess := func(t *testing.T, pid int, n int) {
		t.Helper()

		p, err := os.FindProcess(pid) //nolint:forbidigo
		if err != nil {
			// process is already dead.
			// no need to check if it's dead with Signal(0).
			return
		}
		if err = p.Signal(syscall.Signal(0)); !errors.Is(err, os.ErrProcessDone) { //nolint:forbidigo
			assert.Errorf(t, err, "process #%d should be dead, but exists", n)
		}
	}
	assertProcess(t, b1.pid, 1)
	assertProcess(t, b2.pid, 2)
}

func TestBrowserMultiClose(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withSkipClose(), withLogCache())

	require.NotPanicsf(t, tb.Close, "first call to browser.close should not panic")
	require.NotPanicsf(t, tb.Close, "second call to browser.close should not panic")
	tb.logCache.assertContains(t, "browser.close only once")
}

func TestMultiConnectToSingleBrowser(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withSkipClose())
	defer tb.Close()

	ctx := context.Background()

	b1, err := tb.browserType.Connect(context.Background(), ctx, tb.wsURL)
	require.NoError(t, err)
	bctx1, err := b1.NewContext(nil)
	require.NoError(t, err)
	p1, err := bctx1.NewPage()
	require.NoError(t, err, "failed to create page #1")

	b2, err := tb.browserType.Connect(context.Background(), ctx, tb.wsURL)
	require.NoError(t, err)
	bctx2, err := b2.NewContext(nil)
	require.NoError(t, err)

	err = p1.Close()
	require.NoError(t, err, "failed to close page #1")
	require.NoError(t, bctx1.Close())

	p2, err := bctx2.NewPage()
	require.NoError(t, err, "failed to create page #2")
	err = p2.Close()
	require.NoError(t, err, "failed to close page #2")
}

func TestCloseContext(t *testing.T) {
	t.Parallel()

	t.Run("close_context", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		_, err := tb.NewContext(nil)
		require.NoError(t, err)

		assert.NotNil(t, tb.Context())

		err = tb.CloseContext()
		require.NoError(t, err)

		assert.Nil(t, tb.Context())
	})

	t.Run("err_no_context", func(t *testing.T) {
		t.Parallel()

		tb := newTestBrowser(t)
		assert.Nil(t, tb.Context())
		assert.Error(t, tb.CloseContext())
	})
}

func TestIsolateBrowserContexts(t *testing.T) {
	t.Parallel()
	tb := newTestBrowser(t)

	b1 := tb.Browser
	b2, err := tb.browserType.Connect(context.Background(), tb.context(), tb.wsURL)
	require.NoError(t, err)
	t.Cleanup(b2.Close)

	bctx1, err := b1.NewContext(nil)
	require.NoError(t, err)
	bctx2, err := b2.NewContext(nil)
	require.NoError(t, err)

	// both browser connections will receive onAttachedToTarget events.
	// each Connection value should filter out events that are not related to
	// the browser context it wasn't created from.
	err = tb.run(tb.context(), func() error {
		_, err := bctx1.NewPage()
		return err
	}, func() error {
		_, err := bctx2.NewPage()
		return err
	})
	require.NoError(t, err)

	// assert.Len produces verbose output. so, use our own len.
	bctx1PagesLen := len(bctx1.Pages())
	bctx2PagesLen := len(bctx2.Pages())
	assert.Equalf(t, 1, bctx1PagesLen, "browser context #1 should be attached to a single page, but got %d", bctx1PagesLen)
	assert.Equalf(t, 1, bctx2PagesLen, "browser context #2 should be attached to a single page, but got %d", bctx2PagesLen)
}
