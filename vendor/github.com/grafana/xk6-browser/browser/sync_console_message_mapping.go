package browser

import (
	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
)

// syncMapConsoleMessage is like mapConsoleMessage but returns synchronous functions.
func syncMapConsoleMessage(vu moduleVU, cm *common.ConsoleMessage) mapping {
	rt := vu.Runtime()
	return mapping{
		"args": func() *sobek.Object {
			var (
				margs []mapping
				args  = cm.Args
			)
			for _, arg := range args {
				a := syncMapJSHandle(vu, arg)
				margs = append(margs, a)
			}

			return rt.ToValue(margs).ToObject(rt)
		},
		// page(), text() and type() are defined as
		// functions in order to match Playwright's API
		"page": func() mapping { return syncMapPage(vu, cm.Page) },
		"text": func() string { return cm.Text },
		"type": func() string { return cm.Type },
	}
}
