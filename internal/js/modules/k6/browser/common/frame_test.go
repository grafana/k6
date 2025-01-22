package common

import (
	"context"
	"testing"
	"time"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	"github.com/chromedp/cdproto/cdp"
	"github.com/stretchr/testify/require"
)

// Test calling Frame.document does not panic with a nil document.
// See: Issue #53 for details.
func TestFrameNilDocument(t *testing.T) {
	t.Parallel()

	vu := k6test.NewVU(t)
	log := log.NewNullLogger()

	fm := NewFrameManager(vu.Context(), nil, nil, nil, log)
	frame := NewFrame(vu.Context(), fm, nil, cdp.FrameID("42"), log)

	// frame should not panic with a nil document
	stub := &executionContextTestStub{
		evalFn: func(apiCtx context.Context, opts evalOptions, js string, args ...any) (res any, err error) {
			// return nil to test for panic
			return nil, nil //nolint:nilnil
		},
	}

	// document() waits for the main execution context
	ok := make(chan struct{}, 1)
	go func() {
		frame.setContext(mainWorld, stub)
		ok <- struct{}{}
	}()
	select {
	case <-ok:
	case <-time.After(time.Second):
		require.FailNow(t, "cannot set the main execution context, frame.setContext timed out")
	}

	require.NotPanics(t, func() {
		_, err := frame.document()
		require.Error(t, err)
	})

	// frame gets the document from the evaluate call
	want := &ElementHandle{}
	stub.evalFn = func(
		apiCtx context.Context, opts evalOptions, js string, args ...any,
	) (res any, err error) {
		return want, nil
	}
	got, err := frame.document()
	require.NoError(t, err)
	require.Equal(t, want, got)

	// frame sets documentHandle in the document method
	got = frame.documentHandle
	require.Equal(t, want, got)
}

// See: Issue #177 for details.
func TestFrameManagerFrameAbortedNavigationShouldEmitANonNilPendingDocument(t *testing.T) {
	t.Parallel()

	ctx, log := context.Background(), log.NewNullLogger()

	// add the frame to frame manager
	fm := NewFrameManager(ctx, nil, nil, NewTimeoutSettings(nil), log)
	frame := NewFrame(ctx, fm, nil, cdp.FrameID("42"), log)
	fm.frames[frame.id] = frame

	// listen for frame navigation events
	recv := make(chan Event)
	frame.on(ctx, []string{EventFrameNavigation}, recv)

	// emit the navigation event
	frame.pendingDocument = &DocumentInfo{
		documentID: "42",
	}
	fm.frameAbortedNavigation(frame.id, "any error", frame.pendingDocument.documentID)

	// receive the emitted event and verify that emitted document
	// is not nil.
	e := <-recv
	require.IsType(t, &NavigationEvent{}, e.data, "event should be a navigation event")
	ne := e.data.(*NavigationEvent)
	require.NotNil(t, ne, "event should not be nil")
	require.NotNil(t, ne.newDocument, "emitted document should not be nil")

	// since the navigation is aborted, the aborting frame should have
	// a nil pending document.
	require.Nil(t, frame.pendingDocument)
}

type executionContextTestStub struct {
	ExecutionContext
	evalFn func(
		apiCtx context.Context, opts evalOptions, js string, args ...any,
	) (res any, err error)
}

func (e *executionContextTestStub) eval( // this needs to be a pointer as otherwise it will copy the mutex inside of it
	apiCtx context.Context, opts evalOptions, js string, args ...any,
) (res any, err error) {
	return e.evalFn(apiCtx, opts, js, args...)
}
