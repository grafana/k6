package browser

import (
	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
)

// mapConsoleMessage to the JS module.
func mapConsoleMessage(vu moduleVU, event common.PageOnEvent) mapping {
	rt := vu.Runtime()
	return mapping{
		"args": func() *sobek.Object {
			var (
				margs []mapping
				args  = event.ConsoleMessage.Args
			)
			for _, arg := range args {
				a := mapJSHandle(vu, arg)
				margs = append(margs, a)
			}

			return rt.ToValue(margs).ToObject(rt)
		},
		// page(), text() and type() are defined as
		// functions in order to match Playwright's API
		"page": func() *sobek.Object {
			mp := mapPage(vu, event.ConsoleMessage.Page)
			return rt.ToValue(mp).ToObject(rt)
		},
		"text": func() *sobek.Object {
			return rt.ToValue(event.ConsoleMessage.Text).ToObject(rt)
		},
		"type": func() *sobek.Object {
			return rt.ToValue(event.ConsoleMessage.Type).ToObject(rt)
		},
	}
}
