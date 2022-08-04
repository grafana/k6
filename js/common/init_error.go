package common

// InitContextError is an error that happened during the a test init context
type InitContextError string

// NewInitContextError returns a new InitContextError with the provided message
func NewInitContextError(msg string) InitContextError {
	return (InitContextError)(msg)
}

func (i InitContextError) Error() string {
	return (string)(i)
}

func (i InitContextError) String() string {
	return (string)(i)
}
