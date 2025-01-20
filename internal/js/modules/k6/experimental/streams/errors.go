package streams

import "github.com/grafana/sobek"

func newTypeError(rt *sobek.Runtime, message string) *jsError {
	return newJsError(rt, rt.Get("TypeError"), TypeError, message)
}

func newRangeError(rt *sobek.Runtime, message string) *jsError {
	return newJsError(rt, rt.Get("RangeError"), RangeError, message)
}

func newJsError(rt *sobek.Runtime, base sobek.Value, kind errorKind, message string) *jsError {
	constructor, ok := sobek.AssertConstructor(base)
	if !ok {
		throw(rt, newError(kind, message))
	}

	e, err := constructor(nil, rt.ToValue(message))
	if err != nil {
		throw(rt, newError(kind, message))
	}

	return &jsError{err: e, msg: message}
}

// jsError is a wrapper around a JS error object.
//
// We need to use it because whenever we need to return a [TypeError]
// or a [RangeError], we want to use original JS errors, which can be
// retrieved from Sobek, for instance with: sobek.Runtime.Get("TypeError").
//
// However, that is implemented as a [*sobek.Object], but sometimes we
// need to return that error as a Go [error], or even keep the instance
// in memory to be returned/thrown later.
//
// So, we use this wrapper instead of returning the original JS error.
// Otherwise, we would need to replace everything typed as [error] with
// [any] to be compatible, and that would be a mess.
type jsError struct {
	err *sobek.Object
	msg string
}

func (e *jsError) Error() string {
	return e.msg
}

func (e *jsError) Err() *sobek.Object {
	return e.err
}

func newError(k errorKind, message string) *streamError {
	return &streamError{
		Name:    k.String(),
		Message: message,
		kind:    k,
	}
}

//go:generate enumer -type=errorKind -output errors_gen.go
type errorKind uint8

const (
	// TypeError is thrown when an argument is not of an expected type
	TypeError errorKind = iota + 1

	// RangeError is thrown when an argument is not within the expected range
	RangeError

	// RuntimeError is thrown when an error occurs that was caused by the JS runtime
	// and is not likely caused by the user, but rather the implementation.
	RuntimeError

	// AssertionError is thrown when an assertion fails
	AssertionError

	// NotSupportedError is thrown when a feature is not supported, or not yet implemented
	NotSupportedError
)

type streamError struct {
	// Name contains the name of the error
	Name string `json:"name"`

	// Message contains the error message
	Message string `json:"message"`

	// kind contains the kind of error
	kind errorKind
}

// Ensure that the fsError type implements the Go `error` interface
var _ error = (*streamError)(nil)

func (e *streamError) Error() string {
	return e.Name + ":" + e.Message
}

func throw(rt *sobek.Runtime, err any) {
	if e, ok := err.(*jsError); ok {
		panic(e.Err())
	}

	panic(errToObj(rt, err))
}

func errToObj(rt *sobek.Runtime, err any) sobek.Value {
	// Undefined remains undefined.
	if sobek.IsUndefined(rt.ToValue(err)) {
		return rt.ToValue(err)
	}

	if e, ok := err.(*sobek.Exception); ok {
		return e.Value().ToObject(rt)
	}

	return rt.ToValue(err).ToObject(rt)
}
