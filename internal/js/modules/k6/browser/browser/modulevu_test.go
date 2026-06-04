package browser

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common/autoscreenshot"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// noopPersister discards everything; used in tests where the persistence
// path is irrelevant.
type noopPersister struct{}

func (noopPersister) Persist(_ context.Context, _ string, _ io.Reader) error {
	return nil
}

func TestAfterAction_NoOpWhenDisabled(t *testing.T) {
	t.Parallel()

	// Zero-valued moduleVU: autoScreenshot is nil. afterAction must bail
	// before touching any other field so this case is panic-free even
	// without a real VU.
	var vu moduleVU
	assert.NotPanics(t, func() { vu.afterAction("Test.afterAction") })
}

func TestAfterAction_NoOpWhenStateNil(t *testing.T) {
	t.Parallel()

	// VU exists but has not been activated: vu.State() returns nil. The
	// auto-screenshot mode is enabled but afterAction must skip safely
	// because we are still in init phase.
	vu := k6test.NewVU(t)

	reg := autoscreenshot.NewRegistry(
		autoscreenshot.ModeActions, noopPersister{}, "test", log.NewNullLogger(), true,
	)
	mvu := moduleVU{
		VU:             vu.VU,
		autoScreenshot: reg,
	}
	assert.NotPanics(t, func() { mvu.afterAction("Test.afterAction") })
}

func TestAfterAction_NoOpWhenNoCapturerForIteration(t *testing.T) {
	t.Parallel()

	// VU activated; registry exists; but OnIterStart was never called for
	// this iteration so no Capturer is available. afterAction must skip
	// the capture without error.
	vu := k6test.NewVU(t)
	vu.ActivateVU()

	reg := autoscreenshot.NewRegistry(
		autoscreenshot.ModeActions, noopPersister{}, "test", log.NewNullLogger(), true,
	)
	mvu := moduleVU{
		VU:             vu.VU,
		autoScreenshot: reg,
	}
	assert.NotPanics(t, func() { mvu.afterAction("Test.afterAction") })
}

func TestOnFailure_NoOpWhenDisabled(t *testing.T) {
	t.Parallel()

	// Zero-valued moduleVU: autoScreenshot is nil. onFailure shares the
	// captureOpenPages helper with afterAction; this test guards the
	// distinct entry point from a regression where the mode-gated bail
	// gets moved into the wrong wrapper.
	var vu moduleVU
	assert.NotPanics(t, func() { vu.onFailure("Test.onFailure") })
}
