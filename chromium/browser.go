package chromium

import (
	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common"
)

// Ensure Browser implements the api.Browser interface.
var _ api.Browser = &Browser{}

type Browser struct {
	common.Browser

	// TODO:
	// - add support for service workers
	// - add support for background pages
}
