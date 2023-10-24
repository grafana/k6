package errext_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/errext"
)

func TestFormat(t *testing.T) {
	t.Parallel()

	t.Run("Nil", func(t *testing.T) {
		t.Parallel()
		errorText, fields := errext.Format(nil)
		assert.Equal(t, "", errorText)
		assert.Empty(t, fields)
	})

	t.Run("Simple", func(t *testing.T) {
		t.Parallel()
		errorText, fields := errext.Format(errors.New("simple error"))
		assert.Equal(t, "simple error", errorText)
		assert.Empty(t, fields)
	})

	t.Run("Exception", func(t *testing.T) {
		t.Parallel()
		err := fakeException{error: errors.New("simple error"), stack: "stack trace"}
		errorText, fields := errext.Format(err)
		assert.Equal(t, "stack trace", errorText)
		assert.Empty(t, fields)
	})

	t.Run("Hint", func(t *testing.T) {
		t.Parallel()
		err := errext.WithHint(errors.New("error with hint"), "hint message")
		errorText, fields := errext.Format(err)
		assert.Equal(t, "error with hint", errorText)
		assert.Equal(t, map[string]interface{}{"hint": "hint message"}, fields)
	})

	t.Run("ExceptionWithHint", func(t *testing.T) {
		t.Parallel()
		err := fakeException{error: errext.WithHint(errors.New("error with hint"), "hint message"), stack: "stack trace"}
		errorText, fields := errext.Format(err)
		assert.Equal(t, "stack trace", errorText)
		assert.Equal(t, map[string]interface{}{"hint": "hint message"}, fields)
	})
}

type fakeException struct {
	error
	stack string
	abort errext.AbortReason
}

func (e fakeException) StackTrace() string {
	return e.stack
}

func (e fakeException) AbortReason() errext.AbortReason {
	return e.abort
}

func (e fakeException) Unwrap() error {
	return e.error
}
