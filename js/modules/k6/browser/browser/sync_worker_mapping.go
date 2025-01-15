package browser

import (
	"go.k6.io/k6/js/modules/k6/browser/common"
)

// syncMapWorker is like mapWorker but returns synchronous functions.
func syncMapWorker(_ moduleVU, w *common.Worker) mapping {
	return mapping{
		"url": w.URL(),
	}
}
