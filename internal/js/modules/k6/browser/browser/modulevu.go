package browser

import (
	"context"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"

	k6common "go.k6.io/k6/v2/js/common"
	k6modules "go.k6.io/k6/v2/js/modules"
)

// errInitContext is returned when a browser module API is used in the init
// context, where there is no VU iteration to operate in and VU.State() is nil.
// Reporting it as a plain error keeps such a call from nil-dereferencing
// VU.State() and taking k6 down with it.
// See https://github.com/grafana/k6/issues/6178.
var errInitContext = k6common.NewInitContextError(
	"the browser module can only be used in the iteration context " +
		"(e.g. the default function), not in the init context",
)

// moduleVU carries module specific VU information.
//
// Currently, it is used to carry the VU object to the inner objects and
// promises.
type moduleVU struct {
	k6modules.VU

	*pidRegistry
	*browserRegistry

	*taskQueueRegistry

	filePersister

	testRunID string
}

// browser returns the VU browser instance for the current iteration.
func (vu moduleVU) browser() (*common.Browser, error) {
	// There's no iteration to resolve a browser for in the init context, where
	// State is nil. This is the shared path behind the synchronous browser APIs
	// (isConnected, userAgent, version). See errInitContext.
	if vu.State() == nil {
		return nil, errInitContext
	}
	return vu.getBrowser(vu.State().Iteration)
}

func (vu moduleVU) Context() context.Context {
	// promises and inner objects need the VU object to be
	// able to use k6-core specific functionality.
	//
	// We should not cache the context (especially the init
	// context from the vu that is received from k6 in
	// NewModuleInstance).
	return k6ext.WithVU(vu.VU.Context(), vu)
}
