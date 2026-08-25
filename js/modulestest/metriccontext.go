package modulestest

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/lib"
)

// NewMetricContextTracker returns a tracker for tests that construct a raw Sobek runtime instead of
// a bundle, where k6 installs the production tracker.
func NewMetricContextTracker(state func() *lib.State) sobek.AsyncContextTracker {
	return common.NewMetricContextTracker(state)
}
