package browser

import (
	"go.k6.io/k6/js/modules/k6/browser/common"
)

// mapWorker to the JS module.
func mapWorker(_ moduleVU, w *common.Worker) mapping {
	return mapping{
		"url": w.URL(),
	}
}
