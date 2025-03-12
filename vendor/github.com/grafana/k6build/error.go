package k6build

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrReasonUnknown signals the reason for an WrappedError is unknown
var ErrReasonUnknown = errors.New("reason unknown")

// WrappedError represents an error returned by the build service
// This custom error type facilitates extracting the reason of an error
// by using errors.Unwrap method.
// It also facilitates checking an error (or its reason) using errors.Is by
// comparing the error and its reason.
// This custom type has the following known limitations:
// - A nil WrappedError 'e' will not satisfy errors.Is(e, nil)
// - Is method will not
type WrappedError struct {
	Err    error `json:"error,omitempty"`
	Reason error `json:"reason,omitempty"`
}

// Error returns the Error as a string
func (e *WrappedError) Error() string {
	return fmt.Sprintf("%s: %s", e.Err, e.Reason)
}

// Is returns true if the target error is the same as the WrappedError or its reason
// It attempts several strategies:
// - compare error and reason to target's Error()
// - unwrap the error and reason and compare to target's WrappedError
// - unwrap target and compares to the error recursively
func (e *WrappedError) Is(target error) bool {
	if target == nil {
		return false
	}

	if e.Err.Error() == target.Error() {
		return true
	}

	if e.Reason != nil && e.Reason.Error() == target.Error() {
		return true
	}

	if u := errors.Unwrap(e.Err); u != nil && u.Error() == target.Error() {
		return true
	}

	if u := errors.Unwrap(e.Reason); u != nil && u.Error() == target.Error() {
		return true
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

func wrap(e *jsonError) error {
	if e == nil {
		return nil
	}
	err := errors.New(e.Err)
	if e.Reason == nil {
		return err
	}

	return NewWrappedError(err, wrap(e.Reason))
}

func unwrap(e error) *jsonError {
	if e == nil {
		return nil
	}

	err, ok := AsError(e)
	if !ok {
		return &jsonError{Err: e.Error()}
	}

	return &jsonError{Err: err.Err.Error(), Reason: unwrap(errors.Unwrap(err))}
}

// MarshalJSON implements the json.Marshaler interface for the WrappedError type
func (e *WrappedError) MarshalJSON() ([]byte, error) {
	return json.Marshal(unwrap(e))
}

// UnmarshalJSON implements the json.Unmarshaler interface for the WrappedError type
func (e *WrappedError) UnmarshalJSON(data []byte) error {
	val := jsonError{}

	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}

	e.Err = errors.New(val.Err)
	e.Reason = wrap(val.Reason)
	return nil
}

// NewWrappedError creates an Error from an error and a reason
// If the reason is nil, ErrReasonUnknown is used
func NewWrappedError(err error, reason error) *WrappedError {
	if reason == nil {
		reason = ErrReasonUnknown
	}
	return &WrappedError{
		Err:    err,
		Reason: reason,
	}
}

// AsError returns an error as an Error, if possible
func AsError(e error) (*WrappedError, bool) {
	err := &WrappedError{}
	if !errors.As(e, &err) {
		return nil, false
	}
	return err, true
}
