// Package browser provides an entry point to the browser extension.
package browser

import (
	"github.com/grafana/xk6-browser/browser"

	k6modules "go.k6.io/k6/js/modules"
)

func init() {
	k6modules.Register("k6/x/browser", browser.NewSync())
}
