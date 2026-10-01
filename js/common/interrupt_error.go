package common

import (
	"errors"

	"github.com/grafana/sobek"
)

// UnwrapSobekInterruptedError returns the internal error handled by Sobek.
func UnwrapSobekInterruptedError(err error) error {
	if sobekErr, ok := errors.AsType[*sobek.InterruptedError](err); ok {
		if e, ok := sobekErr.Value().(error); ok {
			return e
		}
	}
	return err
}
