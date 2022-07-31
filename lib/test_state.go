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

// TestRunState contains the pre-init state as well as all of the state and
// options that are necessary for actually running the test.
type TestRunState struct {
	*TestPreInitState

	Options Options
	Runner  Runner // TODO: rename to something better, see type comment

	// TODO: add atlas root node

	// TODO: add other properties that are computed or derived after init, e.g.
	// thresholds?
}
