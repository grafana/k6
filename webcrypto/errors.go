package webcrypto

import (
	"fmt"
)

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
)

type WebCryptoError struct {
	// Code is one of the legacy error code constants, or 0 if none match.
	Code int `json:"code"`

	// Name contains one of the strings associated with an error name.
	Name string `json:"name"`

	// Message represents message or description associated with the given error name.
	Message string `json:"message"`
}

// Error implements the `error` interface, so WebCryptoError are normal Go errors.
func (e *WebCryptoError) Error() string {
	return fmt.Sprintf(e.Name)
}

func NewWebCryptoError(code int, name, message string) *WebCryptoError {
	return &WebCryptoError{
		Code:    code,
		Name:    name,
		Message: message,
	}
}

var _ error = (*WebCryptoError)(nil)
