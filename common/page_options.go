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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/xk6-browser/k6"

	"github.com/chromedp/cdproto/page"
	"github.com/dop251/goja"
)

type PageEmulateMediaOptions struct {
	ColorScheme   ColorScheme   `json:"colorScheme"`
	Media         MediaType     `json:"media"`
	ReducedMotion ReducedMotion `json:"reducedMotion"`
}

type PageReloadOptions struct {
	WaitUntil LifecycleEvent `json:"waitUntil"`
	Timeout   time.Duration  `json:"timeout"`
}

type PageScreenshotOptions struct {
	Clip           *page.Viewport `json:"clip"`
	Path           string         `json:"path"`
	Format         ImageFormat    `json:"format"`
	FullPage       bool           `json:"fullPage"`
	OmitBackground bool           `json:"omitBackground"`
	Quality        int64          `json:"quality"`
}

func NewPageEmulateMediaOptions(defaultMedia MediaType, defaultColorScheme ColorScheme, defaultReducedMotion ReducedMotion) *PageEmulateMediaOptions {
	return &PageEmulateMediaOptions{
		ColorScheme:   defaultColorScheme,
		Media:         defaultMedia,
		ReducedMotion: defaultReducedMotion,
	}
}

func (o *PageEmulateMediaOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6.Runtime(ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "colorScheme":
				o.ColorScheme = ColorScheme(opts.Get(k).String())
			case "media":
				o.Media = MediaType(opts.Get(k).String())
			case "reducedMotion":
				o.ReducedMotion = ReducedMotion(opts.Get(k).String())
			}
		}
	}
	return nil
}

func NewPageReloadOptions(defaultWaitUntil LifecycleEvent, defaultTimeout time.Duration) *PageReloadOptions {
	return &PageReloadOptions{
		WaitUntil: defaultWaitUntil,
		Timeout:   defaultTimeout,
	}
}

func (o *PageReloadOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6.Runtime(ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "waitUntil":
				lifeCycle := opts.Get(k).String()
				if l, ok := lifecycleEventToID[lifeCycle]; ok {
					o.WaitUntil = l
				} else {
					return fmt.Errorf("%q is not a valid lifecycle", lifeCycle)
				}
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
}

func NewPageScreenshotOptions() *PageScreenshotOptions {
	return &PageScreenshotOptions{
		Clip:           nil,
		Path:           "",
		Format:         ImageFormatPNG,
		FullPage:       false,
		OmitBackground: false,
		Quality:        100,
	}
}

func (o *PageScreenshotOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6.Runtime(ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		formatSpecified := false
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "clip":
				var c map[string]float64
				if rt.ExportTo(opts.Get(k), &c) != nil {
					o.Clip = &page.Viewport{
						X:      c["x"],
						Y:      c["y"],
						Width:  c["width"],
						Height: c["height"],
						Scale:  1,
					}
				}
			case "fullPage":
				o.FullPage = opts.Get(k).ToBoolean()
			case "omitBackground":
				o.OmitBackground = opts.Get(k).ToBoolean()
			case "path":
				o.Path = opts.Get(k).String()
			case "quality":
				o.Quality = opts.Get(k).ToInteger()
			case "type":
				if f, ok := imageFormatToID[opts.Get(k).String()]; ok {
					o.Format = f
					formatSpecified = true
				}
			}
		}

		// Infer file format by path if format not explicitly specified (default is PNG)
		if o.Path != "" && !formatSpecified {
			if strings.HasSuffix(o.Path, ".jpg") || strings.HasSuffix(o.Path, ".jpeg") {
				o.Format = ImageFormatJPEG
			}
		}
	}

	return nil
}
