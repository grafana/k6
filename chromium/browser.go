package chromium

import (
	"github.com/grafana/xk6-browser/common"
)

// Browser is the public interface of a CDP browser.
type Browser struct {
	common.Browser

	// TODO:
	// - add support for service workers
	// - add support for background pages
}
