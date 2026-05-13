package log

import (
	"errors"
	"maps"
)

type errWithFieldsError struct {
	error
	fields map[string]any
}

func (e errWithFieldsError) Unwrap() error {
	return e.error
}

// ErrWithFields attaches structured log fields to err.
func ErrWithFields(err error, fields map[string]any) error {
	if err == nil {
		return nil
	}
	if len(fields) == 0 {
		return err
	}
	merged := FieldsFromErr(err)
	maps.Copy(merged, fields)
	return errWithFieldsError{error: err, fields: merged}
}

// FieldsFromErr returns structured log fields attached to err.
func FieldsFromErr(err error) map[string]any {
	fields := map[string]any{}
	var ferr errWithFieldsError
	if errors.As(err, &ferr) {
		maps.Copy(fields, ferr.fields)
	}
	return fields
}
