package common

import (
	"fmt"

	"github.com/chromedp/cdproto/runtime"
)

// Error is a common package error.
type Error string

// Error satisfies the builtin error interface.
func (e Error) Error() string {
	return string(e)
}

// Error types.
const (
	ErrUnexpectedRemoteObjectWithID Error = "cannot extract value when remote object ID is given"
	ErrChannelClosed                Error = "channel closed"
	ErrFrameDetached                Error = "frame detached"
	ErrJSHandleDisposed             Error = "JS handle is disposed"
	ErrJSHandleInvalid              Error = "JS handle is invalid"
	ErrTargetCrashed                Error = "Target has crashed"
	ErrTimedOut                     Error = "timed out"
	ErrWrongExecutionContext        Error = "JS handles can be evaluated only in the context they were created"
)

type BigIntParseError struct {
	err error
}

// Error satisfies the builtin error interface.
func (e BigIntParseError) Error() string {
	return fmt.Sprintf("parsing bigint: %v", e.err)
}

// Is satisfies the builtin error Is interface.
func (e BigIntParseError) Is(target error) bool {
	_, ok := target.(BigIntParseError)
	return ok
}

// Unwrap satisfies the builtin error Unwrap interface.
func (e BigIntParseError) Unwrap() error {
	return e.err
}

type UnserializableValueError struct {
	UnserializableValue runtime.UnserializableValue
}

// Error satisfies the builtin error interface.
func (e UnserializableValueError) Error() string {
	return fmt.Sprintf("unsupported unserializable value: %s", e.UnserializableValue)
}
