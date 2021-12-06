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
	"encoding/json"
	"testing"

	"github.com/grafana/xk6-browser/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserContextOptionsDefaultValues(t *testing.T) {
	t.Parallel()

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

func TestBrowserContextOptionsDefaultViewport(t *testing.T) {
	t.Parallel()

	p := TestBrowser(t).NewPage(nil)
	t.Cleanup(func() { p.Close(nil) })

	viewportSize := p.ViewportSize()
	assert.Equal(t, float64(common.DefaultScreenWidth), viewportSize["width"])
	assert.Equal(t, float64(common.DefaultScreenHeight), viewportSize["height"])
}

func TestBrowserContextOptionsSetViewport(t *testing.T) {
	t.Parallel()

	tb := TestBrowser(t)
	bctx := tb.NewContext(tb.Runtime.ToValue(struct {
		Viewport common.Viewport `js:"viewport"`
	}{
		Viewport: common.Viewport{
			Width:  800,
			Height: 600,
		},
	}))
	t.Cleanup(bctx.Close)
	p := bctx.NewPage()

	viewportSize := p.ViewportSize()
	assert.Equal(t, float64(800), viewportSize["width"])
	assert.Equal(t, float64(600), viewportSize["height"])
}

func TestBrowserContextOptionsExtraHTTPHeaders(t *testing.T) {
	t.Parallel()

	tb := TestBrowser(t)
	bctx := tb.NewContext(tb.Runtime.ToValue(struct {
		ExtraHTTPHeaders map[string]string `js:"extraHTTPHeaders"`
	}{
		ExtraHTTPHeaders: map[string]string{
			"Some-Header": "Some-Value",
		},
	}))
	t.Cleanup(bctx.Close)

	p := bctx.NewPage()
	resp := p.Goto(tb.HTTPMultiBin.ServerHTTP.URL+"/get", nil)

	require.NotNil(t, resp)
	var body struct{ Headers map[string][]string }
	err := json.Unmarshal(resp.Body().Bytes(), &body)
	require.NoError(t, err)
	h := body.Headers["Some-Header"]
	require.NotEmpty(t, h)
	assert.Equal(t, "Some-Value", h[0])
}
