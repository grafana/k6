package execution

// Controller implementations are used to control the k6 execution of a test or
// test suite, either locally or in a distributed environment.
type Controller interface {
	// GetOrCreateData requests the data chunk with the given ID, if it already
	// exists. If it doesn't (i.e. this was the first time this function was
	// called with that ID), the given callback is called and its result and
	// error are saved for the ID and returned for all other calls with it.
	//
	// This is an atomic and single-flight function, so any calls to it while the callback is
	// being executed the same ID will wait for the first call to finish
	// and receive its result.
	//
	// TODO: split apart into `Once()`, `SetData(), `GetData()` and implement
	// the GetOrCreateData() behavior in a helper like the ones below?
	GetOrCreateData(ID string, callback func() ([]byte, error)) ([]byte, error)

	// Signal is used to notify that the current instance has reached the given
	// event ID, or that it has had an error.
	Signal(eventID string, err error) error

	// Subscribe creates a listener for the specified event ID and returns a
	// callback that can wait until all other instances have reached it.
	Subscribe(eventID string) (wait func() error)
}

// SignalAndWait implements a rendezvous point / barrier, a way for all
// instances to reach the same execution point and wait for each other, before
// they all ~simultaneously continue with the execution.
//
// It subscribes for the given event ID, signals that the current instance has
// reached it without an error, and then waits until all other instances have
// reached it or until there is an error in one of them.
func SignalAndWait(c Controller, eventID string) error {
	wait := c.Subscribe(eventID)

	if err := c.Signal(eventID, nil); err != nil {
		return err
	}
	return wait()
}

// SignalErrorOrWait is a helper method that either immediately signals the
// given error and returns it, or it signals nominal completion and waits for
// all other instances to do the same (or signal an error themselves).
func SignalErrorOrWait(c Controller, eventID string, err error) error {
	if err != nil {
		_ = c.Signal(eventID, err)
		return err // return the same error we got
	}
	return SignalAndWait(c, eventID)
}
