package lib

// TimeoutError is used when somethings timeouts
type TimeoutError string

// NewTimeoutError returns a new TimeoutError reporting that timeout has happened at the provieded
// place
func NewTimeoutError(place string) TimeoutError {
	return TimeoutError(place)
}

func (t TimeoutError) String() string {
	return "Timeout during " + (string)(t)
}

func (t TimeoutError) Error() string {
	return t.String()
}
