package common

import (
	"context"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
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

// Parse parses the mouse click options.
func (o *MouseClickOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
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

// Parse parses the mouse double click options.
func (o *MouseDblClickOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
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

// ToMouseClickOptions converts MouseDblClickOptions to a MouseClickOptions.
func (o *MouseDblClickOptions) ToMouseClickOptions() *MouseClickOptions {
	o2 := NewMouseClickOptions()
	o2.Button = o.Button
	o2.ClickCount = 2
	o2.Delay = o.Delay
	return o2
}

func NewMouseDownUpOptions() *MouseDownUpOptions {
	return &MouseDownUpOptions{
		Button:     "left",
		ClickCount: 1,
	}
}

// Parse parses the mouse down/up options.
func (o *MouseDownUpOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
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

// Parse parses the mouse move options.
func (o *MouseMoveOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			if k == "steps" {
				o.Steps = opts.Get(k).ToInteger()
			}
		}
	}
	return nil
}
