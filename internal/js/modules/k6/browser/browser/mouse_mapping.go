package browser

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	k6common "go.k6.io/k6/js/common"
)

func mapMouse(vu moduleVU, m *common.Mouse) mapping {
	rt := vu.Runtime()
	return mapping{
		"click": func(x float64, y float64, opts sobek.Value) *sobek.Promise {
			popts := parseMouseClickOptions(rt, opts)
			return promise(vu, func() (any, error) {
				return nil, m.Click(x, y, popts) //nolint:wrapcheck
			})
		},
		"dblClick": func(x float64, y float64, opts sobek.Value) *sobek.Promise {
			popts := parseMouseDblClickOptions(rt, opts)
			return promise(vu, func() (any, error) {
				return nil, m.DblClick(x, y, popts) //nolint:wrapcheck
			})
		},
		"down": func(opts sobek.Value) *sobek.Promise {
			popts := parseMouseDownUpOptions(rt, opts)
			return promise(vu, func() (any, error) {
				return nil, m.Down(popts) //nolint:wrapcheck
			})
		},
		"up": func(opts sobek.Value) *sobek.Promise {
			popts := parseMouseDownUpOptions(rt, opts)
			return promise(vu, func() (any, error) {
				return nil, m.Up(popts) //nolint:wrapcheck
			})
		},
		"move": func(x float64, y float64, opts sobek.Value) *sobek.Promise {
			popts := parseMouseMoveOptions(rt, opts)
			return promise(vu, func() (any, error) {
				return nil, m.Move(x, y, popts) //nolint:wrapcheck
			})
		},
	}
}

// parseMouseClickOptions parses the mouse click options from a Sobek value.
func parseMouseClickOptions(rt *sobek.Runtime, opts sobek.Value) *common.MouseClickOptions {
	popts := common.NewMouseClickOptions()
	if k6common.IsNullish(opts) {
		return popts
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "button":
			popts.Button = obj.Get(k).String()
		case "clickCount":
			popts.ClickCount = obj.Get(k).ToInteger()
		case "delay":
			popts.Delay = obj.Get(k).ToInteger()
		}
	}

	return popts
}

// parseMouseDblClickOptions parses the mouse double click options from a Sobek value.
func parseMouseDblClickOptions(rt *sobek.Runtime, opts sobek.Value) *common.MouseDblClickOptions {
	popts := common.NewMouseDblClickOptions()
	if k6common.IsNullish(opts) {
		return popts
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "button":
			popts.Button = obj.Get(k).String()
		case "delay":
			popts.Delay = obj.Get(k).ToInteger()
		}
	}

	return popts
}

// parseMouseDownUpOptions parses the mouse down/up options from a Sobek value.
func parseMouseDownUpOptions(rt *sobek.Runtime, opts sobek.Value) *common.MouseDownUpOptions {
	popts := common.NewMouseDownUpOptions()
	if k6common.IsNullish(opts) {
		return popts
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "button":
			popts.Button = obj.Get(k).String()
		case "clickCount":
			popts.ClickCount = obj.Get(k).ToInteger()
		}
	}

	return popts
}

// parseMouseMoveOptions parses the mouse move options from a Sobek value.
func parseMouseMoveOptions(rt *sobek.Runtime, opts sobek.Value) *common.MouseMoveOptions {
	popts := common.NewMouseMoveOptions()
	if k6common.IsNullish(opts) {
		return popts
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		if k == "steps" {
			popts.Steps = obj.Get(k).ToInteger()
		}
	}

	return popts
}
