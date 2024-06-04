package browser

import (
	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapJSHandle to the JS module.
func mapJSHandle(vu moduleVU, jsh common.JSHandleAPI) mapping {
	return mapping{
		"asElement": func() mapping {
			return mapElementHandle(vu, jsh.AsElement())
		},
		"dispose": func() *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, jsh.Dispose() //nolint:wrapcheck
			})
		},
		"evaluate": func(pageFunc goja.Value, gargs ...goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				args := make([]any, 0, len(gargs))
				for _, a := range gargs {
					args = append(args, exportArg(a))
				}
				return jsh.Evaluate(pageFunc.String(), args...) //nolint:wrapcheck
			})
		},
		"evaluateHandle": func(pageFunc goja.Value, gargs ...goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				h, err := jsh.EvaluateHandle(pageFunc.String(), exportArgs(gargs)...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, h), nil
			})
		},
		"getProperties": func() (mapping, error) {
			props, err := jsh.GetProperties()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			dst := make(map[string]any)
			for k, v := range props {
				dst[k] = mapJSHandle(vu, v)
			}
			return dst, nil
		},
		"jsonValue": jsh.JSONValue,
	}
}
