package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	k6common "go.k6.io/k6/js/common"
)

// mapping is a type for mapping our module API to sobek.
// It acts like a bridge and allows adding wildcard methods
// and customization over our API.
type mapping = map[string]any

// mapBrowserToSobek maps the browser API to the JS module.
// The motivation of this mapping was to support $ and $$ wildcard
// methods.
// See issue #661 for more details.
func mapBrowserToSobek(vu moduleVU) *sobek.Object {
	var (
		rt  = vu.Runtime()
		obj = rt.NewObject()
	)
	for k, v := range mapBrowser(vu) {
		err := obj.Set(k, rt.ToValue(v))
		if err != nil {
			k6common.Throw(rt, fmt.Errorf("mapping: %w", err))
		}
	}

	return obj
}
