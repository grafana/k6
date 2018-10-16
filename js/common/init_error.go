package common

// InitError is an error that happened during the a test initiation
type InitError string

// NewInitError returns a new InitError with the provided message
func NewInitError(msg string) InitError {
	return (InitError)(msg)
}

func (i InitError) Error() string {
	return (string)(i)
}

func (i InitError) String() string {
	return (string)(i)
}
