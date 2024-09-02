package execution

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/event"
)

// testAbortKey is the key used to store the abort function for the context of
// an executor. This allows any users of that context or its sub-contexts to
// cancel the whole execution tree, while at the same time providing all of the
// details for why they cancelled it via the attached error.
type testAbortKey struct{}

// EventAbortEmitter is used to abstract the event.System abort method.
// It especially helps for tests where we can hide the implementation
// and use a mock instead.
type EventAbortEmitter interface {
	Emit(event *event.Event) (wait func(context.Context) error)
}

type testAbortController struct {
	cancel context.CancelFunc

	logger logrus.FieldLogger
	lock   sync.Mutex // only the first reason will be kept, other will be logged
	reason error      // see errext package, you can wrap errors to attach exit status, run status, etc.
}

func (tac *testAbortController) abort(ctx context.Context, events EventAbortEmitter, err error) {
	// This is a temporary abort signal. It should be removed once
	// https://github.com/grafana/xk6-browser/issues/1410 is complete.
	waitDone := events.Emit(&event.Event{
		Type: event.Abort,
	})
	// Unlike in run.go where the timeout is 30 minutes, it is being set to
	// 5 seconds here. Since this is a temporary event, it doesn't need to
	// abide by the same rules as the other events. This is only being used
	// by the browser module, and we know it can process the abort event within
	// 5 seconds, this is good enough in the short term.
	waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
	defer waitCancel()
	tac.logger.Infof("abort event fired due to '%s'", err)
	if werr := waitDone(waitCtx); werr != nil {
		tac.logger.WithError(werr).Warn()
	}

	tac.lock.Lock()
	defer tac.lock.Unlock()
	if tac.reason != nil {
		tac.logger.Debugf(
			"test abort with reason '%s' was attempted when the test was already aborted due to '%s'",
			err.Error(), tac.reason.Error(),
		)
		return
	}
	tac.reason = err
	tac.cancel()
}

func (tac *testAbortController) getReason() error {
	tac.lock.Lock()
	defer tac.lock.Unlock()
	return tac.reason
}

// NewTestRunContext returns context.Context that can be aborted by calling the
// returned TestAbortFunc or by calling CancelTestRunContext() on the returned
// context or a sub-context of it. Use this to initialize the context that will
// be passed to the ExecutionScheduler, so `execution.test.abort()` and the REST
// API test stopping both work.
func NewTestRunContext(
	ctx context.Context, logger logrus.FieldLogger,
) (newCtx context.Context, abortTest func(ctx context.Context, events EventAbortEmitter, err error)) {
	ctx, cancel := context.WithCancel(ctx)

	controller := &testAbortController{
		cancel: cancel,
		logger: logger,
	}

	return context.WithValue(ctx, testAbortKey{}, controller), controller.abort
}

// AbortTestRun will cancel the test run context with the given reason if the
// provided context is actually a TestRuncontext or a child of one.
func AbortTestRun(ctx context.Context, events EventAbortEmitter, err error) bool {
	if x := ctx.Value(testAbortKey{}); x != nil {
		if v, ok := x.(*testAbortController); ok {
			v.abort(ctx, events, err)
			return true
		}
	}
	return false
}

// GetCancelReasonIfTestAborted returns a reason the Context was cancelled, if it was
// aborted with these functions. It will return nil if ctx is not an
// TestRunContext (or its children) or if it was never aborted.
func GetCancelReasonIfTestAborted(ctx context.Context) error {
	if x := ctx.Value(testAbortKey{}); x != nil {
		if v, ok := x.(*testAbortController); ok {
			return v.getReason()
		}
	}
	return nil
}
