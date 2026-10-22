package browser

import (
	"context"
	"errors"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"

	k6modules "go.k6.io/k6/v2/js/modules"
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
	iter, err := vu.iteration()
	if err != nil {
		return nil, err
	}
	return vu.getBrowser(iter)
}

// errInitContext is returned when the browser module is used where there is
// no VU state, i.e. in the init context.
var errInitContext = errors.New(
	"the browser module can only be used in the iteration context " +
		"(e.g. the default function), not in the init context",
)

// iteration returns the current iteration number, or errInitContext if the
// VU has no state yet (init context).
func (vu moduleVU) iteration() (int64, error) {
	state := vu.State()
	if state == nil {
		return 0, errInitContext
	}
	return state.Iteration, nil
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
