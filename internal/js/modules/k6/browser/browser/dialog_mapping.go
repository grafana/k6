package browser

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func mapDialog(vu moduleVU, event common.PageEvent) mapping {
	d := event.Dialog
	return finishMapping(mapping{
		"accept": networkCall(func(promptText ...string) *sobek.Promise {
			if !d.Claim() {
				return promise(vu, func() (any, error) { return nil, common.ErrDialogAlreadyHandled })
			}
			return promise(vu, func() (any, error) {
				return nil, d.Accept(promptText...) //nolint:wrapcheck
			})
		}),
		"dismiss": networkCall(func() *sobek.Promise {
			if !d.Claim() {
				return promise(vu, func() (any, error) { return nil, common.ErrDialogAlreadyHandled })
			}
			return promise(vu, func() (any, error) {
				return nil, d.Dismiss() //nolint:wrapcheck
			})
		}),
		"type":         passiveCall(d.Type),
		"message":      passiveCall(d.Message),
		"defaultValue": passiveCall(d.DefaultValue),
		"page": passiveCall(func() mapping {
			return mapPage(vu, d.Page())
		}),
	})
}
