package testutils

import (
	"io"
	"testing"

	"github.com/sirupsen/logrus"
)

// Something that makes the test also be a valid io.Writer, useful for passing it
// as an output for logs and CLI flag help messages...
type testOutput struct{ testing.TB }

func (to testOutput) Write(p []byte) (n int, err error) {
	to.Logf("%s", p)

	return len(p), nil
}

// NewTestOutput returns a simple io.Writer implementation that uses the test's
// logger as an output.
func NewTestOutput(t testing.TB) io.Writer {
	return testOutput{t}
}

func newLogger(t testing.TB, level logrus.Level) *logrus.Logger { //nolint:forbidigo
	l := logrus.New()
	l.SetLevel(level)
	var w io.Writer
	if t == nil {
		w = io.Discard
	} else {
		w = NewTestOutput(t)
	}
	logrus.SetOutput(w)
	return l
}

// NewLogger returns a new logger instance. If the given argument is not nil,
// the logger will log everything using its t.Logf() method. If its nil, all
// messages will be discarded.
func NewLogger(t testing.TB) logrus.FieldLogger {
	return newLogger(t, logrus.InfoLevel)
}

// NewLoggerWithHook calls NewLogger() and attaches a hook with the given
// levels. If no levels are specified, then logrus.AllLevels will be used and
// the lowest log level will be Debug.
func NewLoggerWithHook(t testing.TB, levels ...logrus.Level) (logrus.FieldLogger, *SimpleLogrusHook) {
	maxLevel := logrus.PanicLevel
	if len(levels) == 0 {
		levels = logrus.AllLevels
		maxLevel = logrus.DebugLevel
	} else {
		for _, l := range levels {
			if l > maxLevel {
				maxLevel = l
			}
		}
	}

	l := newLogger(t, maxLevel)
	hook := NewLogHook(levels...)
	l.AddHook(hook)
	return l, hook
}
