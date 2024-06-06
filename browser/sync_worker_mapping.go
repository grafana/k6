package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapWorker is like mapWorker but returns synchronous functions.
func syncMapWorker(_ moduleVU, _ *common.Worker) mapping {
	return nil
}
