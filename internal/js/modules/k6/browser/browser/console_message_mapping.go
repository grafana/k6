package browser

import (
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

// mapConsoleMessage to the JS module.
func mapConsoleMessage(vu moduleVU, event common.PageEvent) mapping {
	cm := event.ConsoleMessage

	return finishMapping(mapping{
		"args": passiveCall(func() []mapping {
			args := cm.Args
			margs := make([]mapping, 0, len(args))
			for _, arg := range args {
				a := mapJSHandle(vu, arg)
				margs = append(margs, a)
			}

			return margs
		}),
		// page(), text() and type() are defined as
		// functions in order to match Playwright's API
		"page": passiveCall(func() mapping {
			return mapPage(vu, cm.Page)
		}),
		"text": passiveCall(func() string {
			return cm.Text
		}),
		"type": passiveCall(func() string {
			return cm.Type
		}),
	})
}
