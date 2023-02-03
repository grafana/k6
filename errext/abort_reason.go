package errext

import "errors"

// AbortReason is used to signal to outputs what type of error caused the test
// run to be stopped prematurely.
type AbortReason uint8

// These are the various reasons why a test might have been aborted prematurely.
const (
	AbortedByUser AbortReason = iota + 1
	AbortedByThreshold
	AbortedByThresholdsAfterTestEnd // TODO: rename?
	AbortedByScriptError
	AbortedByScriptAbort
	AbortedByTimeout
	AbortedByOutput
)

// HasAbortReason is a wrapper around an error with an attached abort reason.
type HasAbortReason interface {
	error
	AbortReason() AbortReason
}

// WithAbortReasonIfNone can attach an abort reason to the given error, if it
// doesn't have one already. It won't do anything if the error already had an
// abort reason attached. Similarly, if there is no error (i.e. the given error
// is nil), it also won't do anything and will return nil.
func WithAbortReasonIfNone(err error, abortReason AbortReason) error {
	if err == nil {
		return nil // No error, do nothing
	}
	var arerr HasAbortReason
	if errors.As(err, &arerr) {
		// The given error already has an abort reason, do nothing
		return err
	}
	return withAbortReason{err, abortReason}
}

type withAbortReason struct {
	error
	abortReason AbortReason
}

func (ar withAbortReason) Unwrap() error {
	return ar.error
}

func (ar withAbortReason) AbortReason() AbortReason {
	return ar.abortReason
}

var _ HasAbortReason = withAbortReason{}
