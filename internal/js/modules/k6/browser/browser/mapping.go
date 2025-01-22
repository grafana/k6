package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"

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

func parseFrameClickOptions(
	ctx context.Context, opts sobek.Value, defaultTimeout time.Duration,
) (*common.FrameClickOptions, error) {
	copts := common.NewFrameClickOptions(defaultTimeout)
	if err := copts.Parse(ctx, opts); err != nil {
		return nil, fmt.Errorf("parsing click options: %w", err)
	}
	return copts, nil
}
