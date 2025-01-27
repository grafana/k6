// Package chromium is responsible for launching a Chrome browser process and managing its lifetime.
package chromium

import (
	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// Browser is the public interface of a CDP browser.
type Browser struct {
	common.Browser

	// TODO:
	// - add support for service workers
	// - add support for background pages
}
