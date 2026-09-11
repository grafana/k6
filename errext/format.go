package errext

import (
	"errors"
)

// Format formats the given error as a message (string) and a map of fields.
// In case of [Exception], it uses the stack trace as the error message.
// In case of [HasHint], it also adds the hint as a field.
func Format(err error) (string, map[string]any) {
	if err == nil {
		return "", nil
	}

	errText := err.Error()
	fields := FieldsFromErr(err)
	if xerr, ok := errors.AsType[Exception](err); ok {
		errText = xerr.StackTrace()
		fields["source"] = "stacktrace"
	}
	if herr, ok := errors.AsType[HasHint](err); ok {
		fields["hint"] = herr.Hint()
	}

	return errText, fields
}
