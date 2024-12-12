package browser

import (
	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
)

// syncMapJSHandle is like mapJSHandle but returns synchronous functions.
func syncMapJSHandle(vu moduleVU, jsh common.JSHandleAPI) mapping {
	rt := vu.Runtime()
	return mapping{
		"asElement": func() *sobek.Object {
			m := syncMapElementHandle(vu, jsh.AsElement())
			return rt.ToValue(m).ToObject(rt)
		},
		"dispose": jsh.Dispose,
		"evaluate": func(pageFunc sobek.Value, gargs ...sobek.Value) (any, error) {
			args := make([]any, 0, len(gargs))
			for _, a := range gargs {
				args = append(args, exportArg(a))
			}
			return jsh.Evaluate(pageFunc.String(), args...) //nolint:wrapcheck
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) (mapping, error) {
			h, err := jsh.EvaluateHandle(pageFunc.String(), exportArgs(gargs)...)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapJSHandle(vu, h), nil
		},
		"getProperties": func() (mapping, error) {
			props, err := jsh.GetProperties()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			dst := make(map[string]any)
			for k, v := range props {
				dst[k] = syncMapJSHandle(vu, v)
			}
			return dst, nil
		},
		"jsonValue": jsh.JSONValue,
	}
}
