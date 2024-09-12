package common

import (
	"errors"

	"github.com/grafana/sobek"
)

// UnwrapSobekInterruptedError returns the internal error handled by Sobek.
func UnwrapSobekInterruptedError(err error) error {
	var sobekErr *sobek.InterruptedError
	if errors.As(err, &sobekErr) {
		if e, ok := sobekErr.Value().(error); ok {
			return e
		}
	}
	return err
}
