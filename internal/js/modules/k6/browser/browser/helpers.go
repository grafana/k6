package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/sobek"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6error"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	k6common "go.k6.io/k6/js/common"
	"go.k6.io/k6/js/promises"
)

func panicIfFatalError(ctx context.Context, err error) {
	if errors.Is(err, k6error.ErrFatal) {
		k6ext.Abortf(ctx, err.Error())
	}
}

// mergeWith merges the Sobek value with the existing Go value.
func mergeWith[T any](rt *sobek.Runtime, src T, v sobek.Value) error {
	if k6common.IsNullish(v) {
		return nil
	}
	return rt.ExportTo(v, &src) //nolint:wrapcheck
}

// exportTo exports the Sobek value to a Go value.
// It returns the zero value of T if obj does not exist in the Sobek runtime.
// It's caller's responsibility to check for nilness.
func exportTo[T any](rt *sobek.Runtime, obj sobek.Value) (T, error) {
	var t T
	if k6common.IsNullish(obj) {
		return t, nil
	}
	err := rt.ExportTo(obj, &t)
	return t, err //nolint:wrapcheck
}

// exportArg exports the value and returns it.
// It returns nil if the value is undefined or null.
func exportArg(gv sobek.Value) any {
	if k6common.IsNullish(gv) {
		return nil
	}
	return gv.Export()
}

// exportArgs returns a slice of exported sobek values.
func exportArgs(gargs []sobek.Value) []any {
	args := make([]any, 0, len(gargs))
	for _, garg := range gargs {
		// leaves a nil garg in the array since users might want to
		// pass undefined or null as an argument to a function
		args = append(args, exportArg(garg))
	}
	return args
}

// sobekEmptyString returns true if a given value is not nil or an empty string.
func sobekEmptyString(v sobek.Value) bool {
	return k6common.IsNullish(v) || strings.TrimSpace(v.String()) == ""
}

// newRegExMatcher returns a function that runs in the JS runtime's event loop
// for pattern matching. It uses ECMAScript RegEx engine for consistency.
//
// It's safe to call this function off of the event loop since the returned
// function gets run in the task queue, ensuring it runs on the event loop.
func newRegExMatcher(ctx context.Context, vu moduleVU, tq *taskqueue.TaskQueue) common.RegExMatcher {
	return func(pattern, str string) (bool, error) {
		return queueTask(ctx, tq, func() (bool, error) {
			v, err := vu.Runtime().RunString(pattern + `.test('` + str + `')`)
			if err != nil {
				return false, fmt.Errorf("evaluating pattern: %w", err)
			}
			return v.ToBoolean(), nil
		})()
	}
}

// promise runs fn in a goroutine and returns a new sobek.Promise.
//   - If fn returns a nil error, resolves the promise with the
//     first result value fn returns.
//   - Otherwise, rejects the promise with the error fn returns.
func promise(vu moduleVU, fn func() (result any, reason error)) *sobek.Promise {
	p, resolve, reject := promises.New(vu)
	go func() {
		v, err := fn()
		if err != nil {
			reject(err)
			return
		}
		resolve(v)
	}()
	return p
}

// queueTask queues the given function fn to run on the given task queue tq.
// The returned future blocks until the task is done and returns the result
// of fn or an error if the context is done before fn completes. It's safe
// not to call the future if you're not interested in the result of fn.
func queueTask[T any](
	ctx context.Context,
	tq *taskqueue.TaskQueue,
	fn func() (T, error),
) (future func() (T, error)) {
	var (
		result T
		err    error
		done   = make(chan struct{})
	)
	tq.Queue(func() error {
		defer close(done)
		result, err = fn()
		return err
	})
	return func() (T, error) {
		select {
		case <-done:
			return result, err
		case <-ctx.Done():
			var zero T
			return zero, fmt.Errorf("running on task queue: %w", common.ContextErr(ctx))
		}
	}
}

// newTaskQueue returns a new [taskqueue.TaskQueue] that is closed after
// the returned cancel function is called or when the VU's context is done.
//
// Do not call this function off of the event loop.
func newTaskQueue(vu moduleVU) (*taskqueue.TaskQueue, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(vu.Context())
	tq := taskqueue.New(vu.RegisterCallback)
	go func() {
		<-ctx.Done()
		tq.Close()
	}()
	return tq, ctx, cancel
}
