package k6ext

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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
