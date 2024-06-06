package common

import (
	"errors"

	"github.com/grafana/sobek"
)

// UnwrapGojaInterruptedError returns the internal error handled by sobek.
func UnwrapGojaInterruptedError(err error) error {
	var gojaErr *sobek.InterruptedError
	if errors.As(err, &gojaErr) {
		if e, ok := gojaErr.Value().(error); ok {
			return e
		}
	}
	return err
}
