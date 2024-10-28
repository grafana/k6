package webcrypto

// ErrorName is a type alias for the name of a WebCryptoError.
//
// Note that it is a type alias, and not a binding, so that it is
// not interpreted as an object by sobek.
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

	// NotImplemented means that we have not implemented the feature yet.
	NotImplemented = "NotImplemented"
)

const (
	errMsgNotExpectedPublicKey  = "given CryptoKey is not a public %s key, it's %T"
	errMsgNotExpectedPrivateKey = "given CryptoKey is not a private %s key, it's %T"
)

// Error represents a custom error emitted by the
// Web Crypto API.
type Error struct {
	// Name contains one of the strings associated with an error name.
	Name string `js:"name"`

	// Message represents message or description associated with the given error name.
	Message string `js:"message"`
}

// Error implements the `error` interface, so WebCryptoError are normal Go errors.
func (e *Error) Error() string {
	return e.Name + ": " + e.Message
}

// NewError returns a new WebCryptoError with the given name and message.
func NewError(name, message string) *Error {
	return &Error{
		Name:    name,
		Message: message,
	}
}

var _ error = (*Error)(nil)
