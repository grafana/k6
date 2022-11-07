package tests

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
)

func TestBrowserNewPage(t *testing.T) {
	b := newTestBrowser(t)
	p := b.NewPage(nil)
	l := len(b.Contexts())
	assert.Equal(t, 1, l, "expected there to be 1 browser context, but found %d", l)

	p2 := b.NewPage(nil)
	l = len(b.Contexts())
	assert.Equal(t, 2, l, "expected there to be 2 browser context, but found %d", l)

	p.Close(nil)
	l = len(b.Contexts())
	assert.Equal(t, 1, l, "expected there to be 1 browser context after first page close, but found %d", l)
	p2.Close(nil)
	l = len(b.Contexts())
	assert.Equal(t, 0, l, "expected there to be 0 browser context after second page close, but found %d", l)
}

func TestTmpDirCleanup(t *testing.T) {
	tmpDirPath := "./"

	err := os.Setenv("TMPDIR", tmpDirPath)
	assert.NoError(t, err)
	defer func() {
		err = os.Unsetenv("TMPDIR")
		assert.NoError(t, err)
	}()

	b := newTestBrowser(t, withSkipClose())
	p := b.NewPage(nil)
	p.Close(nil)

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

	script := `b.on('%s').then(log).catch(log);`

	t.Run("err_wrong_event", func(t *testing.T) {
		t.Parallel()

		b := newTestBrowser(t)
		require.NoError(t, b.vu.Runtime().Set("b", b.Browser))

		err := b.await(func() error {
			_, err := b.runJavaScript(script, "wrongevent")
			return err
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, `unknown browser event: "wrongevent", must be "disconnected"`)
	})

	t.Run("ok_promise_resolved", func(t *testing.T) {
		t.Parallel()

		var (
			b   = newTestBrowser(t, withSkipClose())
			rt  = b.vu.Runtime()
			log []string
		)

		require.NoError(t, rt.Set("b", b.Browser))
		require.NoError(t, rt.Set("log", func(s string) { log = append(log, s) }))

		err := b.await(func() error {
			time.AfterFunc(100*time.Millisecond, b.Browser.Close)
			_, err := b.runJavaScript(script, "disconnected")
			return err
		})
		require.NoError(t, err)
		assert.Contains(t, log, "true")
	})

	t.Run("ok_promise_rejected", func(t *testing.T) {
		t.Parallel()

		var (
			ctx, cancel = context.WithCancel(context.Background())
			b           = newTestBrowser(t, ctx)
			rt          = b.vu.Runtime()
			log         []string
		)

		require.NoError(t, rt.Set("b", b.Browser))
		require.NoError(t, rt.Set("log", func(s string) { log = append(log, s) }))

		err := b.await(func() error {
			time.AfterFunc(100*time.Millisecond, cancel)
			_, err := b.runJavaScript(script, "disconnected")
			return err
		})
		require.NoError(t, err)
		assert.Contains(t, log, "browser.on promise rejected: context canceled")
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
	t.Parallel()

	assertExceptionContains(t, goja.New(), func() {
		lopts := defaultLaunchOpts()
		lopts.Args = []any{"remote-debugging-port=99999"}

		newTestBrowser(t, lopts)
	}, "launching browser: Invalid devtools server port")
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
