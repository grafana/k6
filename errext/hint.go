package errext

import "errors"

// HasHint is a wrapper around an error with an attached user hint. These hints
// can be used to give extra human-readable information about the error,
// including suggestions on how the error can be fixed.
type HasHint interface {
	error
	Hint() string
}

// WithHint is a helper that can attach a hint to the given error. If there is
// no error (i.e. the given error is nil), it won't do anything. If the given
// error already had a hint, this helper will wrap it so that the new hint is
// "new hint (old hint)".
func WithHint(err error, hint string) error {
	if err == nil {
		return nil // No error, do nothing
	}
	return withHint{err, hint}
}

type withHint struct {
	error
	hint string
}

func (wh withHint) Unwrap() error {
	return wh.error
}

func (wh withHint) Hint() string {
	hint := wh.hint
	var oldhint HasHint
	if errors.As(wh.error, &oldhint) {
		// The given error already had a hint, wrap it
		hint = hint + " (" + oldhint.Hint() + ")"
	}

	return hint
}

var _ HasHint = withHint{}
