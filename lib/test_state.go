package lib

import (
	"io"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/event"
	"go.k6.io/k6/lib/trace"
	"go.k6.io/k6/metrics"
)

// TestPreInitState contains all of the state that can be gathered and built
// before the test run is initialized.
type TestPreInitState struct {
	RuntimeOptions RuntimeOptions
	Registry       *metrics.Registry
	BuiltinMetrics *metrics.BuiltinMetrics
	Events         *event.System
	KeyLogger      io.Writer
	LookupEnv      func(key string) (val string, ok bool)
	Logger         logrus.FieldLogger
	TracerProvider *trace.TracerProvider
}

// TestRunState contains the pre-init state as well as all of the state and
// options that are necessary for actually running the test.
type TestRunState struct {
	*TestPreInitState

	Options Options
	Runner  Runner // TODO: rename to something better, see type comment
	RunTags *metrics.TagSet

	// TODO: add other properties that are computed or derived after init, e.g.
	// thresholds?
}
