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
	"testing"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/testutils/browsertest"
	"github.com/stretchr/testify/assert"
)

func TestBrowserContextOptions(t *testing.T) {
	bt := browsertest.NewBrowserTest(t)
	defer bt.Browser.Close()

	t.Run("BrowserContextOptions", func(t *testing.T) {
		t.Run("should have correct default values", func(t *testing.T) { testBrowserContextOptionsDefaultValues(t, bt) })
		t.Run("should correctly set default viewport", func(t *testing.T) { testBrowserContextOptionsDefaultViewport(t, bt) })
		t.Run("should correctly set custom viewport", func(t *testing.T) { testBrowserContextOptionsSetViewport(t, bt) })
	})
}

func testBrowserContextOptionsDefaultValues(t *testing.T, bt *browsertest.BrowserTest) {
	opts := common.NewBrowserContextOptions()
	assert.False(t, opts.AcceptDownloads)
	assert.False(t, opts.BypassCSP)
	assert.Equal(t, common.ColorSchemeLight, opts.ColorScheme)
	assert.Equal(t, 1.0, opts.DeviceScaleFactor)
	assert.Empty(t, opts.ExtraHTTPHeaders)
	assert.Nil(t, opts.Geolocation)
	assert.False(t, opts.HasTouch)
	assert.Nil(t, opts.HttpCredentials)
	assert.False(t, opts.IgnoreHTTPSErrors)
	assert.False(t, opts.IsMobile)
	assert.True(t, opts.JavaScriptEnabled)
	assert.Equal(t, common.DefaultLocale, opts.Locale)
	assert.False(t, opts.Offline)
	assert.Empty(t, opts.Permissions)
	assert.Equal(t, common.ReducedMotionNoPreference, opts.ReducedMotion)
	assert.Equal(t, &common.Screen{Width: common.DefaultScreenWidth, Height: common.DefaultScreenHeight}, opts.Screen)
	assert.Equal(t, "", opts.TimezoneID)
	assert.Equal(t, "", opts.UserAgent)
	assert.Equal(t, &common.Viewport{Width: common.DefaultScreenWidth, Height: common.DefaultScreenHeight}, opts.Viewport)
}

func testBrowserContextOptionsDefaultViewport(t *testing.T, bt *browsertest.BrowserTest) {
	p := bt.Browser.NewPage(nil)
	defer p.Close(nil)

	viewportSize := p.ViewportSize()
	assert.Equal(t, float64(common.DefaultScreenWidth), viewportSize["width"])
	assert.Equal(t, float64(common.DefaultScreenHeight), viewportSize["height"])
}

func testBrowserContextOptionsSetViewport(t *testing.T, bt *browsertest.BrowserTest) {
	bctx := bt.Browser.NewContext(bt.Runtime.ToValue(struct {
		Viewport common.Viewport `js:"viewport"`
	}{
		Viewport: common.Viewport{
			Width:  800,
			Height: 600,
		},
	}))
	defer bctx.Close()
	p := bctx.NewPage()

	viewportSize := p.ViewportSize()
	assert.Equal(t, float64(800), viewportSize["width"])
	assert.Equal(t, float64(600), viewportSize["height"])
}
