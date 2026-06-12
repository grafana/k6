package browser

import "go.k6.io/k6/v2/internal/js/modules/k6/browser/common"

func mapDialog(_ moduleVU, event common.PageEvent) mapping {
	d := event.Dialog
	return mapping{
		"accept":       d.Accept,
		"dismiss":      d.Dismiss,
		"type":         d.Type,
		"message":      d.Message,
		"defaultValue": d.DefaultValue,
	}
}
