package browser

import (
	"github.com/grafana/sobek"
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// mapFrameLocator API to the JS module.
func mapFrameLocator(vu moduleVU, fl *common.FrameLocator) mapping {
	rt := vu.Runtime()
	return mapping{
		"locator": func(selector string, opts sobek.Value) mapping {
			return mapLocator(vu, fl.Locator(selector, parseLocatorOptions(rt, opts)))
		},
	}
}
