package browser

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func mapDialog(vu moduleVU, event common.PageEvent) mapping {
	d := event.Dialog
	return mapping{
		"accept": func(promptText ...string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, d.Accept(promptText...) //nolint:wrapcheck
			})
		},
		"dismiss": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, d.Dismiss() //nolint:wrapcheck
			})
		},
		"type":         d.Type,
		"message":      d.Message,
		"defaultValue": d.DefaultValue,
		"page": func() mapping {
			return mapPage(vu, d.Page())
		},
	}
}
