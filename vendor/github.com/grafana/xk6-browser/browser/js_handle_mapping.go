package browser

import (
	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapJSHandle to the JS module.
func mapJSHandle(vu moduleVU, jsh common.JSHandleAPI) mapping {
	return mapping{
		"asElement": func() mapping {
			return mapElementHandle(vu, jsh.AsElement())
		},
		"dispose": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, jsh.Dispose() //nolint:wrapcheck
			})
		},
		"evaluate": func(pageFunc sobek.Value, gargs ...sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				args := make([]any, 0, len(gargs))
				for _, a := range gargs {
					args = append(args, exportArg(a))
				}
				return jsh.Evaluate(pageFunc.String(), args...) //nolint:wrapcheck
			})
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				h, err := jsh.EvaluateHandle(pageFunc.String(), exportArgs(gargs)...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, h), nil
			})
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
