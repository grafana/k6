package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// syncMapConsoleMessage is like mapConsoleMessage but returns synchronous functions.
func syncMapConsoleMessage(_ moduleVU, _ *common.ConsoleMessage) mapping {
	return nil
}
