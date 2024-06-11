package common

import (
	"errors"

	"github.com/grafana/sobek"
)

// UnwrapGojaInterruptedError returns the internal error handled by Sobek.
func UnwrapGojaInterruptedError(err error) error {
	var sobekErr *sobek.InterruptedError
	if errors.As(err, &sobekErr) {
		if e, ok := sobekErr.Value().(error); ok {
			return e
		}
	}
	return err
}
