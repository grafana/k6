package errext

import (
	"errors"
	"maps"
)

type fieldsError struct {
	error
	fields map[string]any
}

func (e fieldsError) Unwrap() error { return e.error }

// WithFields attaches structured log fields to err.
func WithFields(err error, fields map[string]any) error {
	if err == nil {
		return nil
	}
	if len(fields) == 0 {
		return err
	}
	merged := FieldsFromErr(err)
	maps.Copy(merged, fields)
	return fieldsError{error: err, fields: merged}
}

// FieldsFromErr returns structured log fields attached to err.
func FieldsFromErr(err error) map[string]any {
	fields := map[string]any{}
	if ferr, ok := errors.AsType[fieldsError](err); ok {
		maps.Copy(fields, ferr.fields)
	}
	return fields
}
