package execution

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
)

// testAbortKey is the key used to store the abort function for the context of
// an executor. This allows any users of that context or its sub-contexts to
// cancel the whole execution tree, while at the same time providing all of the
// details for why they cancelled it via the attached error.
type testAbortKey struct{}

type testAbortController struct {
	cancel context.CancelFunc

	logger logrus.FieldLogger
	lock   sync.Mutex // only the first reason will be kept, other will be logged
	reason error      // see errext package, you can wrap errors to attach exit status, run status, etc.
}

func (tac *testAbortController) abort(err error) {
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
) (newCtx context.Context, abortTest func(reason error)) {
	ctx, cancel := context.WithCancel(ctx)

	controller := &testAbortController{
		cancel: cancel,
		logger: logger,
	}

	return context.WithValue(ctx, testAbortKey{}, controller), controller.abort
}

// AbortTestRun will cancel the test run context with the given reason if the
// provided context is actually a TestRuncontext or a child of one.
func AbortTestRun(ctx context.Context, err error) bool {
	if x := ctx.Value(testAbortKey{}); x != nil {
		if v, ok := x.(*testAbortController); ok {
			v.abort(err)
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
