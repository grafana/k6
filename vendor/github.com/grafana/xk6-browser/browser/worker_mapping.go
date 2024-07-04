package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// mapWorker to the JS module.
func mapWorker(_ moduleVU, w *common.Worker) mapping {
	return mapping{
		"url": w.URL(),
	}
}
