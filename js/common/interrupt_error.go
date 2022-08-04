package common

import (
	"errors"

	"github.com/dop251/goja"
)

// UnwrapGojaInterruptedError returns the internal error handled by goja.
func UnwrapGojaInterruptedError(err error) error {
	var gojaErr *goja.InterruptedError
	if errors.As(err, &gojaErr) {
		if e, ok := gojaErr.Value().(error); ok {
			return e
		}
	}
	return err
}
