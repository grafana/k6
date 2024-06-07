package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// syncMapElementHandle is like mapElementHandle but returns synchronous functions.
func syncMapElementHandle(vu moduleVU, eh *common.ElementHandle) mapping { //nolint:gocognit,cyclop,funlen
	rt := vu.Runtime()
	maps := mapping{
		"boundingBox": eh.BoundingBox,
		"check":       eh.Check,
		"click": func(opts sobek.Value) (*sobek.Promise, error) {
			ctx := vu.Context()

			popts := common.NewElementHandleClickOptions(eh.Timeout())
			if err := popts.Parse(ctx, opts); err != nil {
				return nil, fmt.Errorf("parsing element click options: %w", err)
			}

			return k6ext.Promise(vu.Context(), func() (any, error) {
				err := eh.Click(popts)
				return nil, err //nolint:wrapcheck
			}), nil
		},
		"contentFrame": func() (mapping, error) {
			f, err := eh.ContentFrame()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapFrame(vu, f), nil
		},
		"dblclick": eh.Dblclick,
		"dispatchEvent": func(typ string, eventInit sobek.Value) error {
			return eh.DispatchEvent(typ, exportArg(eventInit)) //nolint:wrapcheck
		},
		"fill":  eh.Fill,
		"focus": eh.Focus,
		"getAttribute": func(name string) (any, error) {
			v, ok, err := eh.GetAttribute(name)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			if !ok {
				return nil, nil //nolint:nilnil
			}
			return v, nil
		},
		"hover":      eh.Hover,
		"innerHTML":  eh.InnerHTML,
		"innerText":  eh.InnerText,
		"inputValue": eh.InputValue,
		"isChecked":  eh.IsChecked,
		"isDisabled": eh.IsDisabled,
		"isEditable": eh.IsEditable,
		"isEnabled":  eh.IsEnabled,
		"isHidden":   eh.IsHidden,
		"isVisible":  eh.IsVisible,
		"ownerFrame": func() (mapping, error) {
			f, err := eh.OwnerFrame()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapFrame(vu, f), nil
		},
		"press": eh.Press,
		"screenshot": func(opts sobek.Value) (*sobek.ArrayBuffer, error) {
			ctx := vu.Context()

			popts := common.NewElementHandleScreenshotOptions(eh.Timeout())
			if err := popts.Parse(ctx, opts); err != nil {
				return nil, fmt.Errorf("parsing frame screenshot options: %w", err)
			}

			bb, err := eh.Screenshot(popts, vu.filePersister)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			ab := rt.NewArrayBuffer(bb)

			return &ab, nil
		},
		"scrollIntoViewIfNeeded": eh.ScrollIntoViewIfNeeded,
		"selectOption":           eh.SelectOption,
		"selectText":             eh.SelectText,
		"setInputFiles":          eh.SetInputFiles,
		"tap": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewElementHandleTapOptions(eh.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing element tap options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, eh.Tap(popts) //nolint:wrapcheck
			}), nil
		},
		"textContent": func() (any, error) {
			v, ok, err := eh.TextContent()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			if !ok {
				return nil, nil //nolint:nilnil
			}
			return v, nil
		},
		"type":                eh.Type,
		"uncheck":             eh.Uncheck,
		"waitForElementState": eh.WaitForElementState,
		"waitForSelector": func(selector string, opts sobek.Value) (mapping, error) {
			eh, err := eh.WaitForSelector(selector, opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapElementHandle(vu, eh), nil
		},
	}
	maps["$"] = func(selector string) (mapping, error) {
		eh, err := eh.Query(selector, common.StrictModeOff)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		// ElementHandle can be null when the selector does not match any elements.
		// We do not want to map nil elementHandles since the expectation is a
		// null result in the test script for this case.
		if eh == nil {
			return nil, nil //nolint:nilnil
		}
		ehm := syncMapElementHandle(vu, eh)

		return ehm, nil
	}
	maps["$$"] = func(selector string) ([]mapping, error) {
		ehs, err := eh.QueryAll(selector)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		var mehs []mapping
		for _, eh := range ehs {
			ehm := syncMapElementHandle(vu, eh)
			mehs = append(mehs, ehm)
		}
		return mehs, nil
	}

	jsHandleMap := syncMapJSHandle(vu, eh)
	for k, v := range jsHandleMap {
		maps[k] = v
	}

	return maps
}
