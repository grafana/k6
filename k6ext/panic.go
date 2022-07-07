package k6ext

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	k6common "go.k6.io/k6/js/common"
)

// Panic will cause a panic with the given error which will shut
// the application down. Before panicking, it will find the
// browser process from the context and kill it if it still exists.
// TODO: test.
func Panic(ctx context.Context, format string, a ...interface{}) {
	rt := Runtime(ctx)
	if rt == nil {
		// this should never happen unless a programmer error
		panic("no k6 JS runtime in context")
	}
	if len(a) > 0 {
		err, ok := a[len(a)-1].(error)
		if ok {
			a[len(a)-1] = &userFriendlyError{err}
		}
	}
	defer k6common.Throw(rt, fmt.Errorf(format, a...))

	pid := GetProcessID(ctx)
	if pid == 0 {
		// this should never happen unless a programmer error
		panic("no browser process ID in context")
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		// optimistically return and don't kill the process
		return
	}
	// no need to check the error for waiting the process to release
	// its resources or whether we could kill it as we're already
	// dying.
	_ = p.Release()
	_ = p.Kill()
}

type userFriendlyError struct{ err error }

func (e *userFriendlyError) Unwrap() error { return e.err }

func (e *userFriendlyError) Error() string {
	switch {
	default:
		return e.err.Error()
	case e.err == nil:
		return ""
	case errors.Is(e.err, context.DeadlineExceeded):
		return strings.ReplaceAll(e.err.Error(), context.DeadlineExceeded.Error(), "timed out")
	case errors.Is(e.err, context.Canceled):
		return "canceled"
	}
}
