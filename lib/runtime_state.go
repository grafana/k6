package lib

import (
	"io"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/metrics"
)

// RuntimeState represents what is mostly needed during the running of a test.
//
// TODO: since this has nothing to do with the goja JS "runtime", maybe we
// should rename it to something more appropriate? e.g. TestRunState?
type RuntimeState struct {
	RuntimeOptions RuntimeOptions
	// TODO maybe have a struct `Metrics` with `Registry` and `Builtin` ?
	Registry       *metrics.Registry
	BuiltinMetrics *metrics.BuiltinMetrics
	KeyLogger      io.WriteCloser

	// TODO: replace with logrus.FieldLogger when all of the tests can be fixed
	Logger *logrus.Logger
}
