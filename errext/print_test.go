package errext_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/errext"
)

func TestFprint(t *testing.T) {
	t.Parallel()

	init := func() (*bytes.Buffer, logrus.FieldLogger) {
		var buf bytes.Buffer
		logger := logrus.New()
		logger.Out = &buf
		return &buf, logger
	}

	t.Run("Nil", func(t *testing.T) {
		t.Parallel()
		buf, logger := init()
		errext.Fprint(logger, nil)
		assert.Equal(t, "", buf.String())
	})

	t.Run("Simple", func(t *testing.T) {
		t.Parallel()
		buf, logger := init()
		errext.Fprint(logger, errors.New("simple error"))
		assert.Contains(t, buf.String(), "level=error msg=\"simple error\"")
	})

	t.Run("Exception", func(t *testing.T) {
		t.Parallel()
		buf, logger := init()
		err := fakeException{error: errors.New("simple error"), stack: "stack trace"}
		errext.Fprint(logger, err)
		assert.Contains(t, buf.String(), "level=error msg=\"stack trace\"")
	})

	t.Run("Hint", func(t *testing.T) {
		t.Parallel()
		buf, logger := init()
		err := errext.WithHint(errors.New("error with hint"), "hint message")
		errext.Fprint(logger, err)
		assert.Contains(t, buf.String(), "level=error msg=\"error with hint\" hint=\"hint message\"")
	})

	t.Run("ExceptionWithHint", func(t *testing.T) {
		t.Parallel()
		buf, logger := init()
		err := fakeException{error: errext.WithHint(errors.New("error with hint"), "hint message"), stack: "stack trace"}
		errext.Fprint(logger, err)
		assert.Contains(t, buf.String(), "level=error msg=\"stack trace\" hint=\"hint message\"")
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
