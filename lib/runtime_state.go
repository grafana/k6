package lib

import (
	"io"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/metrics"
)

// RuntimeState represents what is mostly needed during the running of a test
type RuntimeState struct {
	RuntimeOptions RuntimeOptions
	// TODO maybe have a struct `Metrics` with `Registry` and `Builtin` ?
	Registry       *metrics.Registry
	BuiltinMetrics *metrics.BuiltinMetrics
	KeyLogger      io.Writer
	Logger         *logrus.Logger
}
