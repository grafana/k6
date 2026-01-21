package browser

import (
	"fmt"
	"reflect"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	k6common "go.k6.io/k6/js/common"
)

func mapMouse(vu moduleVU, m *common.Mouse) mapping {
	rt := vu.Runtime()
	return mapping{
		"click": func(x float64, y float64, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseMouseClickOptions(rt, opts)
			if err != nil {
				return nil, fmt.Errorf("parsing mouse click options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, m.Click(x, y, popts) //nolint:wrapcheck
			}), nil
		},
		"dblClick": func(x float64, y float64, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseMouseDblClickOptions(rt, opts)
			if err != nil {
				return nil, fmt.Errorf("parsing mouse double click options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, m.DblClick(x, y, popts) //nolint:wrapcheck
			}), nil
		},
		"down": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseMouseDownUpOptions(rt, opts)
			if err != nil {
				return nil, fmt.Errorf("parsing mouse down options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, m.Down(popts) //nolint:wrapcheck
			}), nil
		},
		"up": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseMouseDownUpOptions(rt, opts)
			if err != nil {
				return nil, fmt.Errorf("parsing mouse up options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, m.Up(popts) //nolint:wrapcheck
			}), nil
		},
		"move": func(x float64, y float64, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseMouseMoveOptions(rt, opts)
			if err != nil {
				return nil, fmt.Errorf("parsing mouse move options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, m.Move(x, y, popts) //nolint:wrapcheck
			}), nil
		},
	}
}

// parseMouseClickOptions parses the mouse click options from a Sobek value.
func parseMouseClickOptions(rt *sobek.Runtime, opts sobek.Value) (*common.MouseClickOptions, error) {
	popts := common.NewMouseClickOptions()
	if k6common.IsNullish(opts) {
		return popts, nil
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		v := obj.Get(k)
		switch k {
		case "button":
			popts.Button = v.String()
		case "clickCount":
			switch v.ExportType().Kind() {
			case reflect.Int64, reflect.Float64:
				popts.ClickCount = v.ToInteger()
			default:
				return nil, fmt.Errorf("clickCount must be a number, got %s", v.ExportType().Kind())
			}
		case "delay":
			switch v.ExportType().Kind() {
			case reflect.Int64, reflect.Float64:
				popts.Delay = v.ToInteger()
			default:
				return nil, fmt.Errorf("delay must be a number, got %s", v.ExportType().Kind())
			}
		}
	}

	return popts, nil
}

// parseMouseDblClickOptions parses the mouse double click options from a Sobek value.
func parseMouseDblClickOptions(rt *sobek.Runtime, opts sobek.Value) (*common.MouseDblClickOptions, error) {
	popts := common.NewMouseDblClickOptions()
	if k6common.IsNullish(opts) {
		return popts, nil
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		v := obj.Get(k)
		switch k {
		case "button":
			popts.Button = v.String()
		case "delay":
			switch v.ExportType().Kind() {
			case reflect.Int64, reflect.Float64:
				popts.Delay = v.ToInteger()
			default:
				return nil, fmt.Errorf("delay must be a number, got %s", v.ExportType().Kind())
			}
		}
	}

	return popts, nil
}

// parseMouseDownUpOptions parses the mouse down/up options from a Sobek value.
func parseMouseDownUpOptions(rt *sobek.Runtime, opts sobek.Value) (*common.MouseDownUpOptions, error) {
	popts := common.NewMouseDownUpOptions()
	if k6common.IsNullish(opts) {
		return popts, nil
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		v := obj.Get(k)
		switch k {
		case "button":
			popts.Button = v.String()
		case "clickCount":
			switch v.ExportType().Kind() {
			case reflect.Int64, reflect.Float64:
				popts.ClickCount = v.ToInteger()
			default:
				return nil, fmt.Errorf("clickCount must be a number, got %s", v.ExportType().Kind())
			}
		}
	}

	return popts, nil
}

// parseMouseMoveOptions parses the mouse move options from a Sobek value.
func parseMouseMoveOptions(rt *sobek.Runtime, opts sobek.Value) (*common.MouseMoveOptions, error) {
	popts := common.NewMouseMoveOptions()
	if k6common.IsNullish(opts) {
		return popts, nil
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		if k == "steps" {
			v := obj.Get(k)
			switch v.ExportType().Kind() {
			case reflect.Int64, reflect.Float64:
				popts.Steps = v.ToInteger()
			default:
				return nil, fmt.Errorf("steps must be a number, got %s", v.ExportType().Kind())
			}
		}
	}

	return popts, nil
}
