package common

// InterruptError is an error that halts engine execution
type InterruptError struct {
	Reason string
}

func (i InterruptError) Error() string {
	return i.Reason
}

// AbortTest is emitted when a test script calls abortTest() without arguments
var AbortTest = &InterruptError{
	Reason: "abortTest() was called in a script",
}

// AbortTest is emitted when a test script calls abortTest() without arguments
// in the init context
var AbortTestInitContext = &InterruptError{
	Reason: "Using abortTest() in the init context is not supported",
}

// IsInteruptError returns true if err is *InterruptError
func IsInteruptError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*InterruptError)
	return ok
}
