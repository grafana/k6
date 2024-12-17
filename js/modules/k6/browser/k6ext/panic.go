package k6ext

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/errext"
	k6common "go.k6.io/k6/js/common"
)

// Abort will shutdown the whole test run. This should
// only be used from the sobek mapping layer. It is only
// to be used when an error will occur in all iterations,
// so it's permanent.
func Abort(ctx context.Context, format string, a ...any) {
	failFunc := func(rt *sobek.Runtime, a ...any) {
		reason := fmt.Errorf(format, a...).Error()
		rt.Interrupt(&errext.InterruptError{Reason: reason})
	}
	sharedPanic(ctx, failFunc, a...)
}

// Panic will cause a panic with the given error which will stop
// the current iteration. Before panicking, it will find the
// browser process from the context and kill it if it still exists.
// TODO: test.
func Panic(ctx context.Context, format string, a ...any) {
	failFunc := func(rt *sobek.Runtime, a ...any) {
		k6common.Throw(rt, fmt.Errorf(format, a...))
	}
	sharedPanic(ctx, failFunc, a...)
}

func sharedPanic(ctx context.Context, failFunc func(rt *sobek.Runtime, a ...any), a ...any) {
	rt := Runtime(ctx)
	if rt == nil {
		// this should never happen unless a programmer error
		panic("no k6 JS runtime in context")
	}
	// get a user-friendly error if the err is not already so.
	if len(a) > 0 {
		var (
			uerr    *UserFriendlyError
			err, ok = a[len(a)-1].(error)
		)
		if ok && !errors.As(err, &uerr) {
			a[len(a)-1] = &UserFriendlyError{Err: err}
		}
	}
	defer failFunc(rt, a...)

	// TODO: Remove this after moving k6ext.Panic into the mapping layer.
	pidder, ok := GetVU(ctx).(interface {
		Pids() []int
	})
	if !ok {
		// we're running in a test, let's skip killing the process.
		return
	}
	for _, pid := range pidder.Pids() {
		p, err := os.FindProcess(pid) //nolint:forbidigo
		if err != nil {
			// optimistically skip and don't kill the process
			continue
		}
		// no need to check the error for whether we could kill it as
		// we're already dying.
		_ = p.Kill()
	}
}

// UserFriendlyError maps an internal error to an error that users
// can easily understand.
type UserFriendlyError struct {
	Err     error
	Timeout time.Duration // prints "timed out after Ns" error
}

func (e *UserFriendlyError) Unwrap() error { return e.Err }

func (e *UserFriendlyError) Error() string {
	switch {
	default:
		return e.Err.Error()
	case e.Err == nil:
		return ""
	case errors.Is(e.Err, context.DeadlineExceeded):
		s := "timed out"
		if t := e.Timeout; t != 0 {
			s += fmt.Sprintf(" after %s", t)
		}
		return strings.ReplaceAll(e.Err.Error(), context.DeadlineExceeded.Error(), s)
	case errors.Is(e.Err, context.Canceled):
		return "canceled"
	}
}
