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

	"github.com/grafana/xk6-browser/k6"

	"github.com/dop251/goja"
)

type MouseClickOptions struct {
	Button     string `json:"button"`
	ClickCount int64  `json:"clickCount"`
	Delay      int64  `json:"delay"`
}

type MouseDblClickOptions struct {
	Button string `json:"button"`
	Delay  int64  `json:"delay"`
}

type MouseDownUpOptions struct {
	Button     string `json:"button"`
	ClickCount int64  `json:"clickCount"`
}

type MouseMoveOptions struct {
	Steps int64 `json:"steps"`
}

func NewMouseClickOptions() *MouseClickOptions {
	return &MouseClickOptions{
		Button:     "left",
		ClickCount: 1,
		Delay:      0,
	}
}

func (o *MouseClickOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6.Runtime(ctx)
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
			}
		}
	}
	return nil
}

func (o *MouseClickOptions) ToMouseDownUpOptions() *MouseDownUpOptions {
	o2 := NewMouseDownUpOptions()
	o2.Button = o.Button
	o2.ClickCount = o.ClickCount
	return o2
}

func NewMouseDblClickOptions() *MouseDblClickOptions {
	return &MouseDblClickOptions{
		Button: "left",
		Delay:  0,
	}
}

func (o *MouseDblClickOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6.Runtime(ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "button":
				o.Button = opts.Get(k).String()
			case "delay":
				o.Delay = opts.Get(k).ToInteger()
			}
		}
	}
	return nil
}

func (o *MouseDblClickOptions) ToMouseDownUpOptions() *MouseDownUpOptions {
	o2 := NewMouseDownUpOptions()
	o2.Button = o.Button
	return o2
}

func NewMouseDownUpOptions() *MouseDownUpOptions {
	return &MouseDownUpOptions{
		Button:     "left",
		ClickCount: 1,
	}
}

func (o *MouseDownUpOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6.Runtime(ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "button":
				o.Button = opts.Get(k).String()
			case "clickCount":
				o.ClickCount = opts.Get(k).ToInteger()
			}
		}
	}
	return nil
}

func NewMouseMoveOptions() *MouseMoveOptions {
	return &MouseMoveOptions{
		Steps: 1,
	}
}

func (o *MouseMoveOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6.Runtime(ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "steps":
				o.Steps = opts.Get(k).ToInteger()
			}
		}
	}
	return nil
}
