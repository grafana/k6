package browser

import (
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// mapConsoleMessage to the JS module.
func mapConsoleMessage(vu moduleVU, event common.PageOnEvent) mapping {
	cm := event.ConsoleMessage

	return mapping{
		"args": func() []mapping {
			var (
				margs []mapping
				args  = cm.Args
			)
			for _, arg := range args {
				a := mapJSHandle(vu, arg)
				margs = append(margs, a)
			}

			return margs
		},
		// page(), text() and type() are defined as
		// functions in order to match Playwright's API
		"page": func() mapping {
			return mapPage(vu, cm.Page)
		},
		"text": func() string {
			return cm.Text
		},
		"type": func() string {
			return cm.Type
		},
	}
}
