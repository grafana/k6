package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

// mapJSHandle to the JS module.
func mapJSHandle(vu moduleVU, jsh common.JSHandleAPI) mapping {
	return finishMapping(newJSHandleMapping(vu, jsh))
}

func newJSHandleMapping(vu moduleVU, jsh common.JSHandleAPI) mapping {
	return mapping{
		"asElement": passiveCall(func() mapping {
			return mapElementHandle(vu, jsh.AsElement())
		}),
		"dispose": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, jsh.Dispose()
			})
		}),
		"evaluate": networkCall(func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluate requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				return jsh.Evaluate(funcString, gopts...)
			}), nil
		}),
		"evaluateHandle": networkCall(func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
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
		}),
		"getProperties": passiveCall(func() *sobek.Promise {
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
		}),
		"jsonValue": passiveCall(func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return jsh.JSONValue() //nolint:wrapcheck
			})
		}),
	}
}
