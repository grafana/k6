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

package common

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"golang.org/x/net/context"
)

// BrowserContextOptions stores browser context options
type BrowserContextOptions struct {
	AcceptDownloads   bool
	BypassCSP         bool
	ColorScheme       ColorScheme
	DeviceScaleFactor float64
	ExtraHTTPHeaders  map[string]string
	Geolocation       *Geolocation
	HasTouch          bool
	HttpCredentials   *Credentials
	IgnoreHTTPSErrors bool
	IsMobile          bool
	JavaScriptEnabled bool
	Locale            string
	Offline           bool
	Permissions       []string
	ReducedMotion     ReducedMotion
	Screen            *Screen
	TimezoneID        string
	UserAgent         string
	VideosPath        string
	Viewport          *Viewport
}

// NewBrowserContextOptions creates a default set of browser context options
func NewBrowserContextOptions() *BrowserContextOptions {
	return &BrowserContextOptions{
		AcceptDownloads:   false,
		BypassCSP:         false,
		ColorScheme:       ColorSchemeLight,
		DeviceScaleFactor: 1.0,
		ExtraHTTPHeaders:  make(map[string]string),
		Geolocation:       nil,
		HasTouch:          false,
		HttpCredentials:   nil,
		IgnoreHTTPSErrors: false,
		IsMobile:          false,
		JavaScriptEnabled: true,
		Locale:            DefaultLocale,
		Offline:           false,
		Permissions:       make([]string, 0),
		ReducedMotion:     ReducedMotionNoPreference,
		Screen:            &Screen{Width: DefaultScreenWidth, Height: DefaultScreenHeight},
		TimezoneID:        "",
		UserAgent:         "",
		Viewport:          &Viewport{Width: DefaultScreenWidth, Height: DefaultScreenHeight},
	}
}

func (b *BrowserContextOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := common.GetRuntime(ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "acceptDownloads":
				b.AcceptDownloads = opts.Get(k).ToBoolean()
			case "bypassCSP":
				b.BypassCSP = opts.Get(k).ToBoolean()
			case "colorScheme":
				switch ColorScheme(opts.Get(k).String()) {
				case "light":
					b.ColorScheme = ColorSchemeLight
				case "dark":
					b.ColorScheme = ColorSchemeDark
				default:
					b.ColorScheme = ColorSchemeNoPreference
				}
			case "deviceScaleFactor":
				b.DeviceScaleFactor = opts.Get(k).ToFloat()
			case "extraHTTPHeaders":
				headers := opts.Get(k).ToObject(rt)
				for _, k := range headers.Keys() {
					b.ExtraHTTPHeaders[k] = opts.Get(k).String()
				}
			case "geolocation":
				geolocation := NewGeolocation()
				if err := geolocation.Parse(ctx, opts.Get(k).ToObject(rt)); err != nil {
					return err
				}
				b.Geolocation = geolocation
			case "hasTouch":
				b.HasTouch = opts.Get(k).ToBoolean()
			case "httpCredentials":
				credentials := NewCredentials()
				if err := credentials.Parse(ctx, opts.Get(k).ToObject(rt)); err != nil {
					return err
				}
				b.HttpCredentials = credentials
			case "ignoreHTTPSErrors":
				b.IgnoreHTTPSErrors = opts.Get(k).ToBoolean()
			case "isMobile":
				b.IsMobile = opts.Get(k).ToBoolean()
			case "javaScriptEnabled":
				b.JavaScriptEnabled = opts.Get(k).ToBoolean()
			case "locale":
				b.Locale = opts.Get(k).String()
			case "offline":
				b.Offline = opts.Get(k).ToBoolean()
			case "permissions":
				permissions := opts.Get(k).Export().([]string)
				b.Permissions = append(b.Permissions, permissions...)
			case "reducedMotion":
				switch ReducedMotion(opts.Get(k).String()) {
				case "reduce":
					b.ReducedMotion = ReducedMotionReduce
				default:
					b.ReducedMotion = ReducedMotionNoPreference
				}
			case "screen":
				screen := NewScreen()
				if err := screen.Parse(ctx, opts.Get(k).ToObject(rt)); err != nil {
					return err
				}
				b.Screen = screen
			case "timezoneID":
				b.TimezoneID = opts.Get(k).String()
			case "userAgent":
				b.UserAgent = opts.Get(k).String()
			case "viewport":
				viewport := NewViewport()
				if err := viewport.Parse(ctx, opts.Get(k).ToObject(rt)); err != nil {
					return err
				}
				b.Viewport = viewport
			}
		}
	}
	return nil
}
