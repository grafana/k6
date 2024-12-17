package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	k6common "go.k6.io/k6/js/common"
)

// syncMapBrowserToSobek maps the browser API to the JS module as a
// synchronous version.
func syncMapBrowserToSobek(vu moduleVU) *sobek.Object {
	var (
		rt  = vu.Runtime()
		obj = rt.NewObject()
	)
	for k, v := range syncMapBrowser(vu) {
		err := obj.Set(k, rt.ToValue(v))
		if err != nil {
			k6common.Throw(rt, fmt.Errorf("mapping: %w", err))
		}
	}

	return obj
}
