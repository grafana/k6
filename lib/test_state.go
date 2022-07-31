package lib

import (
	"io"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/metrics"
)

// TestPreInitState contains all of the state that can be gathered and built
// before the test run is initialized.
type TestPreInitState struct {
	RuntimeOptions RuntimeOptions
	Registry       *metrics.Registry
	BuiltinMetrics *metrics.BuiltinMetrics
	KeyLogger      io.WriteCloser

	// TODO: replace with logrus.FieldLogger when all of the tests can be fixed
	Logger *logrus.Logger
}
