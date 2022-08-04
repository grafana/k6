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

// NewLogger Returns new logger that will log to the testing.TB.Logf
func NewLogger(t testing.TB) *logrus.Logger {
	l := logrus.New()
	logrus.SetOutput(NewTestOutput(t))

	return l
}
