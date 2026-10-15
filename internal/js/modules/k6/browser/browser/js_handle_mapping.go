package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

// mapJSHandle to the JS module.
func mapJSHandle(vu moduleVU, jsh common.JSHandleAPI) mapping {
	// Register network operations against the handle's owning page so that
	// network-classified calls (evaluate/evaluateHandle) invoked directly on a
	// standalone handle are attributed like they are on element handles. Page()
	// is not part of the JS-facing JSHandleAPI, so reach it via the concrete
	// implementations (BaseJSHandle/ElementHandle). It is nil for worker or
	// isolated-world handles, in which case withPageNetworkCalls falls back to
	// finishing the mapping without a network-operation begin.
	var page *common.Page
	if ph, ok := jsh.(interface{ Page() *common.Page }); ok {
		page = ph.Page()
	}
	return withPageNetworkCalls(vu, page, newJSHandleMapping(vu, jsh))
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
