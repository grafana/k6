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
	"strings"
	"time"

	"github.com/dop251/goja"
)

type ElementHandleBaseOptions struct {
	Force       bool          `json:"force"`
	NoWaitAfter bool          `json:"noWaitAfter"`
	Timeout     time.Duration `json:"timeout"`
}

type ElementHandleBasePointerOptions struct {
	ElementHandleBaseOptions
	Position *Position `json:"position"`
	Trial    bool      `json:"trial"`
}

// ScrollPosition is a parameter for scrolling an element.
type ScrollPosition string

const (
	// ScrollPositionStart scrolls an element at the top of its parent.
	ScrollPositionStart ScrollPosition = "start"
	// ScrollPositionCenter scrolls an element at the center of its parent.
	ScrollPositionCenter ScrollPosition = "center"
	// ScrollPositionEnd scrolls an element at the end of its parent.
	ScrollPositionEnd ScrollPosition = "end"
	// ScrollPositionNearest scrolls an element at the nearest position of its parent.
	ScrollPositionNearest ScrollPosition = "nearest"
)

// ScrollIntoViewOptions change the behavior of ScrollIntoView.
// See: https://developer.mozilla.org/en-US/docs/Web/API/Element/scrollIntoView
type ScrollIntoViewOptions struct {
	// Block defines vertical alignment.
	// One of start, center, end, or nearest.
	// Defaults to start.
	Block ScrollPosition `json:"block"`

	// Inline defines horizontal alignment.
	// One of start, center, end, or nearest.
	// Defaults to nearest.
	Inline ScrollPosition `json:"inline"`
}

type ElementHandleCheckOptions struct {
	ElementHandleBasePointerOptions
}

type ElementHandleClickOptions struct {
	ElementHandleBasePointerOptions
	Button     string   `json:"button"`
	ClickCount int64    `json:"clickCount"`
	Delay      int64    `json:"delay"`
	Modifiers  []string `json:"modifiers"`
}

type ElementHandleDblclickOptions struct {
	ElementHandleBasePointerOptions
	Button    string   `json:"button"`
	Delay     int64    `json:"delay"`
	Modifiers []string `json:"modifiers"`
}

type ElementHandleHoverOptions struct {
	ElementHandleBasePointerOptions
	Modifiers []string `json:"modifiers"`
}

type ElementHandlePressOptions struct {
	Delay       int64         `json:"delay"`
	NoWaitAfter bool          `json:"noWaitAfter"`
	Timeout     time.Duration `json:"timeout"`
}

type ElementHandleScreenshotOptions struct {
	Path           string        `json:"path"`
	Format         ImageFormat   `json:"format"`
	OmitBackground bool          `json:"omitBackground"`
	Quality        int64         `json:"quality"`
	Timeout        time.Duration `json:"timeout"`
}

type ElementHandleSetCheckedOptions struct {
	ElementHandleBasePointerOptions
	Strict bool `json:"strict"`
}

type ElementHandleTapOptions struct {
	ElementHandleBasePointerOptions
	Modifiers []string `json:"modifiers"`
}

type ElementHandleTypeOptions struct {
	Delay       int64         `json:"delay"`
	NoWaitAfter bool          `json:"noWaitAfter"`
	Timeout     time.Duration `json:"timeout"`
}

type ElementHandleWaitForElementStateOptions struct {
	Timeout time.Duration `json:"timeout"`
}

func NewElementHandleBaseOptions(defaultTimeout time.Duration) *ElementHandleBaseOptions {
	return &ElementHandleBaseOptions{
		Force:       false,
		NoWaitAfter: false,
		Timeout:     defaultTimeout,
	}
}

func (o *ElementHandleBaseOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "force":
				o.Force = opts.Get(k).ToBoolean()
			case "noWaitAfter":
				o.NoWaitAfter = opts.Get(k).ToBoolean()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
}

func NewElementHandleBasePointerOptions(defaultTimeout time.Duration) *ElementHandleBasePointerOptions {
	return &ElementHandleBasePointerOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
		Position:                 nil,
		Trial:                    false,
	}
}

func (o *ElementHandleBasePointerOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "position":
				var p map[string]float64
				o.Position = &Position{}
				if rt.ExportTo(opts.Get(k), &p) != nil {
					o.Position.X = p["x"]
					o.Position.Y = p["y"]
				}
			case "trial":
				o.Trial = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewElementHandleCheckOptions(defaultTimeout time.Duration) *ElementHandleCheckOptions {
	return &ElementHandleCheckOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
	}
}

func (o *ElementHandleCheckOptions) Parse(ctx context.Context, opts goja.Value) error {
	return o.ElementHandleBasePointerOptions.Parse(ctx, opts)
}

func NewElementHandleClickOptions(defaultTimeout time.Duration) *ElementHandleClickOptions {
	return &ElementHandleClickOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Button:                          "left",
		ClickCount:                      1,
		Delay:                           0,
		Modifiers:                       []string{},
	}
}

func (o *ElementHandleClickOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "button":
				o.Button = opts.Get(k).String()
			case "clickCount":
				o.ClickCount = opts.Get(k).ToInteger()
			case "delay":
				o.Delay = opts.Get(k).ToInteger()
			case "modifiers":
				var m []string
				if err := rt.ExportTo(opts.Get(k), &m); err != nil {
					return err
				}
				o.Modifiers = m
			}
		}
	}
	return nil
}

func (o *ElementHandleClickOptions) ToMouseClickOptions() *MouseClickOptions {
	o2 := NewMouseClickOptions()
	o2.Button = o.Button
	o2.ClickCount = o.ClickCount
	o2.Delay = o.Delay
	return o2
}

func NewElementHandleDblclickOptions(defaultTimeout time.Duration) *ElementHandleDblclickOptions {
	return &ElementHandleDblclickOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Button:                          "left",
		Delay:                           0,
		Modifiers:                       []string{},
	}
}

func (o *ElementHandleDblclickOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "button":
				o.Button = opts.Get(k).String()
			case "delay":
				o.Delay = opts.Get(k).ToInteger()
			case "modifiers":
				var m []string
				if err := rt.ExportTo(opts.Get(k), &m); err != nil {
					return err
				}
				o.Modifiers = m
			}
		}
	}
	return nil
}

func (o *ElementHandleDblclickOptions) ToMouseClickOptions() *MouseClickOptions {
	o2 := NewMouseClickOptions()
	o2.Button = o.Button
	o2.ClickCount = 2
	o2.Delay = o.Delay
	return o2
}

func NewElementHandleHoverOptions(defaultTimeout time.Duration) *ElementHandleHoverOptions {
	return &ElementHandleHoverOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Modifiers:                       []string{},
	}
}

func (o *ElementHandleHoverOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "modifiers":
				var m []string
				if err := rt.ExportTo(opts.Get(k), &m); err != nil {
					return err
				}
				o.Modifiers = m
			}
		}
	}
	return nil
}

func NewElementHandlePressOptions(defaultTimeout time.Duration) *ElementHandlePressOptions {
	return &ElementHandlePressOptions{
		Delay:       0,
		NoWaitAfter: false,
		Timeout:     defaultTimeout,
	}
}

func (o *ElementHandlePressOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "delay":
				o.Delay = opts.Get(k).ToInteger()
			case "noWaitAfter":
				o.NoWaitAfter = opts.Get(k).ToBoolean()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
}

func (o *ElementHandlePressOptions) ToBaseOptions() *ElementHandleBaseOptions {
	o2 := ElementHandleBaseOptions{}
	o2.Force = false
	o2.NoWaitAfter = o.NoWaitAfter
	o2.Timeout = o.Timeout
	return &o2
}

func NewElementHandleScreenshotOptions(defaultTimeout time.Duration) *ElementHandleScreenshotOptions {
	return &ElementHandleScreenshotOptions{
		Path:           "",
		Format:         ImageFormatPNG,
		OmitBackground: false,
		Quality:        100,
		Timeout:        defaultTimeout,
	}
}

func (o *ElementHandleScreenshotOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		formatSpecified := false
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
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
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
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

func NewElementHandleSetCheckedOptions(defaultTimeout time.Duration) *ElementHandleSetCheckedOptions {
	return &ElementHandleSetCheckedOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Strict:                          false,
	}
}

func (o *ElementHandleSetCheckedOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()

	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}

	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewElementHandleTapOptions(defaultTimeout time.Duration) *ElementHandleTapOptions {
	return &ElementHandleTapOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Modifiers:                       []string{},
	}
}

func (o *ElementHandleTapOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "modifiers":
				var m []string
				if err := rt.ExportTo(opts.Get(k), &m); err != nil {
					return err
				}
				o.Modifiers = m
			}
		}
	}
	return nil
}

func NewElementHandleTypeOptions(defaultTimeout time.Duration) *ElementHandleTypeOptions {
	return &ElementHandleTypeOptions{
		Delay:       0,
		NoWaitAfter: false,
		Timeout:     defaultTimeout,
	}
}

func (o *ElementHandleTypeOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "delay":
				o.Delay = opts.Get(k).ToInteger()
			case "noWaitAfter":
				o.NoWaitAfter = opts.Get(k).ToBoolean()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
}

func (o *ElementHandleTypeOptions) ToBaseOptions() *ElementHandleBaseOptions {
	o2 := ElementHandleBaseOptions{}
	o2.Force = false
	o2.NoWaitAfter = o.NoWaitAfter
	o2.Timeout = o.Timeout
	return &o2
}

func NewElementHandleWaitForElementStateOptions(defaultTimeout time.Duration) *ElementHandleWaitForElementStateOptions {
	return &ElementHandleWaitForElementStateOptions{
		Timeout: defaultTimeout,
	}
}

func (o *ElementHandleWaitForElementStateOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
}
