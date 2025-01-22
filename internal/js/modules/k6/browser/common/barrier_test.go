package common

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

func TestBarrier(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	log := log.NewNullLogger()

	timeoutSettings := NewTimeoutSettings(nil)
	frameManager := NewFrameManager(ctx, nil, nil, timeoutSettings, log)
	frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"), log)

	barrier := NewBarrier()
	barrier.AddFrameNavigation(frame)
	frame.emit(EventFrameNavigation, "some data")

	err := barrier.Wait(ctx)
	require.Nil(t, err)
}
