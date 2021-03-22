package common

type InterruptError struct {
	Reason string
}

func (i InterruptError) Error() string {
	return i.Reason
}

var AbortTest = &InterruptError{
	Reason: "abortTest() was called in a script",
}

var AbortTestInitContext = &InterruptError{
	Reason: "Using abortTest() in the init context is not supported",
}

func IsInteruptError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*InterruptError)
	return ok
}
