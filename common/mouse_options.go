package common

import (
	"context"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/k6ext"
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
	rt := k6ext.Runtime(ctx)
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
	rt := k6ext.Runtime(ctx)
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
	rt := k6ext.Runtime(ctx)
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
	rt := k6ext.Runtime(ctx)
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
