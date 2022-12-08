package webcrypto

import (
	"fmt"
)

// ErrorName is a type alias for the name of a WebCryptoError.
//
// Note that it is a type alias, and not a binding, so that it is
// not interpreted as an object by goja.
type ErrorName = string

const (
	// DataError represents the DataError error.
	DataError = "DataError"

	// ImplementationError represents the ImplementationError error.
	// It is thrown when the error is likely a bug in our implementation.
	ImplementationError = "ImplementationError"

	// InvalidAccessError represents the InvalidAccessError error.
	InvalidAccessError = "InvalidAccessError"

	// NotSupportedError represents the NotSupportedError error.
	NotSupportedError ErrorName = "NotSupportedError"

	// OperationError represents the OperationError error.
	OperationError = "OperationError"

	// SyntaxError represents the SyntaxError error.
	SyntaxError = "SyntaxError"

	// TypeMismatchError represents the TypeMismatchError error.
	TypeMismatchError = "TypeMismatchError"

	// TypeError represents the TypeError error.
	TypeError = "TypeError"

	// QuotaExceededError is the error thrown if the byteLength of a typedArray
	// exceeds 65,536.
	QuotaExceededError = "QuotaExceededError"
)

// Error represents a custom error emitted by the
// Web Crypto API.
type Error struct {
	// Code is one of the legacy error code constants, or 0 if none match.
	Code int `json:"code"`

	// Name contains one of the strings associated with an error name.
	Name string `json:"name"`

	// Message represents message or description associated with the given error name.
	Message string `json:"message"`
}

// Error implements the `error` interface, so WebCryptoError are normal Go errors.
func (e *Error) Error() string {
	return fmt.Sprintf(e.Name)
}

// NewError returns a new WebCryptoError with the given name and message.
func NewError(code int, name, message string) *Error {
	return &Error{
		Code:    code,
		Name:    name,
		Message: message,
	}
}

var _ error = (*Error)(nil)
