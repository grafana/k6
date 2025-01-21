package tests

import (
	_ "embed"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

func TestBrowserContextOptionsDefaultValues(t *testing.T) {
	t.Parallel()

	opts := common.DefaultBrowserContextOptions()
	assert.False(t, opts.AcceptDownloads)
	assert.Empty(t, opts.DownloadsPath)
	assert.False(t, opts.BypassCSP)
	assert.Equal(t, common.ColorSchemeLight, opts.ColorScheme)
	assert.Equal(t, 1.0, opts.DeviceScaleFactor)
	assert.Empty(t, opts.ExtraHTTPHeaders)
	assert.Nil(t, opts.Geolocation)
	assert.False(t, opts.HasTouch)
	assert.True(t, opts.HTTPCredentials.IsEmpty())
	assert.False(t, opts.IgnoreHTTPSErrors)
	assert.False(t, opts.IsMobile)
	assert.True(t, opts.JavaScriptEnabled)
	assert.Equal(t, common.DefaultLocale, opts.Locale)
	assert.False(t, opts.Offline)
	assert.Empty(t, opts.Permissions)
	assert.Equal(t, common.ReducedMotionNoPreference, opts.ReducedMotion)
	assert.Equal(t, common.Screen{Width: common.DefaultScreenWidth, Height: common.DefaultScreenHeight}, opts.Screen)
	assert.Equal(t, "", opts.TimezoneID)
	assert.Equal(t, "", opts.UserAgent)
	assert.Equal(t, common.Viewport{Width: common.DefaultScreenWidth, Height: common.DefaultScreenHeight}, opts.Viewport)
}

func TestBrowserContextOptionsDefaultViewport(t *testing.T) {
	t.Parallel()

	p := newTestBrowser(t).NewPage(nil)

	viewportSize := p.ViewportSize()
	assert.Equal(t, float64(common.DefaultScreenWidth), viewportSize["width"])
	assert.Equal(t, float64(common.DefaultScreenHeight), viewportSize["height"])
}

func TestBrowserContextOptionsSetViewport(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t)
	opts := common.DefaultBrowserContextOptions()
	opts.Viewport = common.Viewport{
		Width:  800,
		Height: 600,
	}
	bctx, err := tb.NewContext(opts)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := bctx.Close(); err != nil {
			t.Log("closing browser context:", err)
		}
	})
	p, err := bctx.NewPage()
	require.NoError(t, err)

	viewportSize := p.ViewportSize()
	assert.Equal(t, float64(800), viewportSize["width"])
	assert.Equal(t, float64(600), viewportSize["height"])
}

func TestBrowserContextOptionsExtraHTTPHeaders(t *testing.T) {
	t.Parallel()

	tb := newTestBrowser(t, withHTTPServer())

	opts := common.DefaultBrowserContextOptions()
	opts.ExtraHTTPHeaders = map[string]string{
		"Some-Header": "Some-Value",
	}
	bctx, err := tb.NewContext(opts)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := bctx.Close(); err != nil {
			t.Log("closing browser context:", err)
		}
	})

	p, err := bctx.NewPage()
	require.NoError(t, err)

	err = tb.awaitWithTimeout(time.Second*5, func() error {
		opts := &common.FrameGotoOptions{
			Timeout: common.DefaultTimeout,
		}
		resp, err := p.Goto(
			tb.url("/get"),
			opts,
		)
		if err != nil {
			return err
		}
		require.NotNil(t, resp)

		responseBody, err := resp.Body()
		require.NoError(t, err)

		var body struct{ Headers map[string][]string }
		require.NoError(t, json.Unmarshal(responseBody, &body))

		h := body.Headers["Some-Header"]
		require.NotEmpty(t, h)
		assert.Equal(t, "Some-Value", h[0])

		return nil
	})
	require.NoError(t, err)
}
