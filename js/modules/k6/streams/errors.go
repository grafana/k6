package streams

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
