package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// mapJSHandle to the JS module.
func mapJSHandle(vu moduleVU, jsh common.JSHandleAPI) mapping {
	return mapping{
		"asElement": func() mapping {
			return mapElementHandle(vu, jsh.AsElement())
		},
		"dispose": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, jsh.Dispose()
			})
		},
		"evaluate": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluate requires a page function")
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				args := make([]any, 0, len(gargs))
				for _, a := range gargs {
					args = append(args, exportArg(a))
				}
				return jsh.Evaluate(pageFunc.String(), args...)
			}), nil
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluateHandle requires a page function")
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				h, err := jsh.EvaluateHandle(pageFunc.String(), exportArgs(gargs)...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, h), nil
			}), nil
		},
		"getProperties": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
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
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return jsh.JSONValue() //nolint:wrapcheck
			})
		},
	}
}
