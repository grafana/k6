package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

// mapJSHandle to the JS module.
func mapJSHandle(vu moduleVU, jsh common.JSHandleAPI) mapping {
	return mapping{
		"asElement": func() any {
			// BaseJSHandle.AsElement() is nil when the remote object is not a
			// DOM node. Mapping that as an ElementHandle produces closures that
			// nil-deref on first use (click, boundingBox, …) and crash the process.
			// Playwright returns null here; $ / $$ already follow the same rule.
			eh := jsh.AsElement()
			if eh == nil {
				return nil
			}
			return mapElementHandle(vu, eh)
		},
		"dispose": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, jsh.Dispose()
			})
		},
		"evaluate": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluate requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				return jsh.Evaluate(funcString, gopts...)
			}), nil
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluateHandle requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				h, err := jsh.EvaluateHandle(funcString, gopts...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, h), nil
			}), nil
		},
		"getProperties": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				props, err := jsh.GetProperties()
				if err != nil {
					return nil, err //nolint:wrapcheck
				}

				dst := make(map[string]any)
				for k, v := range props {
					dst[k] = mapJSHandle(vu, v)
				}
				return dst, nil
			})
		},
		"jsonValue": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return jsh.JSONValue() //nolint:wrapcheck
			})
		},
	}
}
