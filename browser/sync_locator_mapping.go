package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// syncMapLocator is like mapLocator but returns synchronous functions.
func syncMapLocator(vu moduleVU, lo *common.Locator) mapping { //nolint:funlen
	return mapping{
		"clear": func(opts sobek.Value) error {
			ctx := vu.Context()

			copts := common.NewFrameFillOptions(lo.Timeout())
			if err := copts.Parse(ctx, opts); err != nil {
				return fmt.Errorf("parsing clear options: %w", err)
			}

			return lo.Clear(copts) //nolint:wrapcheck
		},
		"click": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameClickOptions(vu.Context(), opts, lo.Timeout())
			if err != nil {
				return nil, err
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Click(popts) //nolint:wrapcheck
			}), nil
		},
		"dblclick":   lo.Dblclick,
		"check":      lo.Check,
		"uncheck":    lo.Uncheck,
		"isChecked":  lo.IsChecked,
		"isEditable": lo.IsEditable,
		"isEnabled":  lo.IsEnabled,
		"isDisabled": lo.IsDisabled,
		"isVisible":  lo.IsVisible,
		"isHidden":   lo.IsHidden,
		"fill":       lo.Fill,
		"focus":      lo.Focus,
		"getAttribute": func(name string, opts sobek.Value) (any, error) {
			v, ok, err := lo.GetAttribute(name, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			if !ok {
				return nil, nil
			}
			return v, nil
		},
		"innerHTML": lo.InnerHTML,
		"innerText": lo.InnerText,
		"textContent": func(opts sobek.Value) (any, error) {
			v, ok, err := lo.TextContent(opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			if !ok {
				return nil, nil
			}
			return v, nil
		},
		"inputValue":   lo.InputValue,
		"selectOption": lo.SelectOption,
		"press":        lo.Press,
		"type":         lo.Type,
		"hover":        lo.Hover,
		"tap": func(opts sobek.Value) (*sobek.Promise, error) {
			copts := common.NewFrameTapOptions(lo.DefaultTimeout())
			if err := copts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing locator tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, lo.Tap(copts) //nolint:wrapcheck
			}), nil
		},
		"dispatchEvent": func(typ string, eventInit, opts sobek.Value) error {
			popts := common.NewFrameDispatchEventOptions(lo.DefaultTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return fmt.Errorf("parsing locator dispatch event options: %w", err)
			}
			return lo.DispatchEvent(typ, exportArg(eventInit), popts) //nolint:wrapcheck
		},
		"waitFor": lo.WaitFor,
	}
}
