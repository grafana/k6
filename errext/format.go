package errext

import (
	"errors"
)

// Format formats the given error as a message (string) and a map of fields.
// In case of [Exception], it uses the stack trace as the error message.
// In case of [HasHint], it also adds the hint as a field.
func Format(err error) (string, map[string]interface{}) {
	if err == nil {
		return "", nil
	}

	errText := err.Error()
	var xerr Exception
	if errors.As(err, &xerr) {
		errText = xerr.StackTrace()
	}

	fields := make(map[string]interface{})
	var herr HasHint
	if errors.As(err, &herr) {
		fields["hint"] = herr.Hint()
	}

	return errText, fields
}
