package browser

import "go.k6.io/k6/v2/internal/js/modules/k6/browser/common"

// mapDialog maps the Dialog event to the JS module.
func mapDialog(_ moduleVU, event common.PageEvent) mapping {
	d := event.Dialog
	return mapping{
		"accept":  func() error { return d.Accept() },
		"dismiss": func() error { return d.Dismiss() },
	}
}
