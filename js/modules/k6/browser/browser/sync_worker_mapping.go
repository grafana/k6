package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapWorker is like mapWorker but returns synchronous functions.
func syncMapWorker(_ moduleVU, w *common.Worker) mapping {
	return mapping{
		"url": w.URL(),
	}
}
