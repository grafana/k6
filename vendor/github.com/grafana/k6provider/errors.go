package k6provider

import (
	"encoding/json"
	"errors"
	"fmt"
)

var errReasonUnknown = errors.New("reason unknown")

// WrappedError represents an error returned by the build service.
// This custom error type facilitates extracting the reason of an error
// by using errors.Unwrap method.
// It also facilitates checking an error (or its reason) using errors.Is by
// comparing the error and its reason.
type WrappedError struct {
	Err    error `json:"error,omitempty"`
	Reason error `json:"reason,omitempty"`
}

// Error returns the Error as a string
func (e *WrappedError) Error() string {
	if e == nil {
		return "<nil>"
	}
	errStr := "<nil>"
	if e.Err != nil {
		errStr = e.Err.Error()
	}
	reasonStr := "<nil>"
	if e.Reason != nil {
		reasonStr = e.Reason.Error()
	}
	return fmt.Sprintf("%s: %s", errStr, reasonStr)
}

// Is returns true if the target error is the same as the WrappedError or its reason
func (e *WrappedError) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}

	if e.Err != nil && e.Err.Error() == target.Error() {
		return true
	}

	if e.Reason != nil && e.Reason.Error() == target.Error() {
		return true
	}

	if e.Err != nil {
		if u := errors.Unwrap(e.Err); u != nil && u.Error() == target.Error() {
			return true
		}
	}

	if e.Reason != nil {
		if u := errors.Unwrap(e.Reason); u != nil && u.Error() == target.Error() {
			return true
		}
	}

	return e.Is(errors.Unwrap(target))
}

// Unwrap returns the underlying reason for the WrappedError
func (e *WrappedError) Unwrap() error {
	return e.Reason
}

type jsonError struct {
	Err    string     `json:"error,omitempty"`
	Reason *jsonError `json:"reason,omitempty"`
}

func wrapJSONError(e *jsonError) error {
	if e == nil {
		return nil
	}
	err := errors.New(e.Err)
	if e.Reason == nil {
		return err
	}

	return NewWrappedError(err, wrapJSONError(e.Reason))
}

func unwrapToJSON(e error) *jsonError {
	if e == nil {
		return nil
	}

	err, ok := AsWrappedError(e)
	if !ok {
		return &jsonError{Err: e.Error()}
	}

	return &jsonError{Err: err.Err.Error(), Reason: unwrapToJSON(errors.Unwrap(err))}
}

// MarshalJSON implements the json.Marshaler interface for the WrappedError type
func (e *WrappedError) MarshalJSON() ([]byte, error) {
	return json.Marshal(unwrapToJSON(e))
}

// UnmarshalJSON implements the json.Unmarshaler interface for the WrappedError type
func (e *WrappedError) UnmarshalJSON(data []byte) error {
	val := jsonError{}

	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}

	e.Err = errors.New(val.Err)
	e.Reason = wrapJSONError(val.Reason)
	return nil
}

// NewWrappedError creates an Error from an error and a reason.
// If the reason is nil, errReasonUnknown is used.
func NewWrappedError(err error, reason error) *WrappedError {
	if reason == nil {
		reason = errReasonUnknown
	}
	return &WrappedError{
		Err:    err,
		Reason: reason,
	}
}

// AsWrappedError returns an error as a WrappedError, if possible
func AsWrappedError(e error) (*WrappedError, bool) {
	err := &WrappedError{}
	if !errors.As(e, &err) {
		return nil, false
	}
	return err, true
}
